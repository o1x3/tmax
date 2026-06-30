// Package update checks GitHub Releases for a newer tmax and can replace the
// running binary in place. The launch-time check is throttled to once a day via
// a small cache file and is fully skippable with TMAX_NO_UPDATE_CHECK.
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Repo is the GitHub "owner/name" releases are pulled from.
const Repo = "o1x3/tmax"

// checkInterval throttles the launch-time check so we hit the network at most
// once per window.
const checkInterval = 24 * time.Hour

// ErrUpToDate is returned by SelfUpdate when the latest release is already
// installed.
var ErrUpToDate = errors.New("already up to date")

// Release is the slice of the GitHub API response we care about.
type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Latest fetches the most recent published release.
func Latest(ctx context.Context) (*Release, error) {
	url := "https://api.github.com/repos/" + Repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github returned %s", resp.Status)
	}
	var r Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Check returns the latest version tag and whether it is newer than current.
// It is best-effort and throttled: when the cached check is fresh it never
// touches the network, and any error yields ("", false) rather than noise.
func Check(current string) (latest string, newer bool) {
	if os.Getenv("TMAX_NO_UPDATE_CHECK") != "" {
		return "", false
	}
	st := loadState()
	if time.Since(st.CheckedAt) < checkInterval {
		return st.Latest, isNewer(current, st.Latest)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	st.CheckedAt = time.Now()
	if rel, err := Latest(ctx); err == nil {
		st.Latest = rel.TagName
	}
	saveState(st) // record the attempt either way so we don't retry every launch
	return st.Latest, isNewer(current, st.Latest)
}

// SelfUpdate downloads the latest release archive for this OS/arch, verifies its
// checksum, and atomically replaces the running executable. Returns the new
// version, or ErrUpToDate if nothing newer exists.
func SelfUpdate(current string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rel, err := Latest(ctx)
	if err != nil {
		return "", err
	}
	if current != "dev" && !isNewer(current, rel.TagName) {
		return rel.TagName, ErrUpToDate
	}

	asset := fmt.Sprintf("tmax_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	var binURL, sumURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case asset:
			binURL = a.URL
		case "checksums.txt":
			sumURL = a.URL
		}
	}
	if binURL == "" {
		return "", fmt.Errorf("no prebuilt binary for %s/%s in %s", runtime.GOOS, runtime.GOARCH, rel.TagName)
	}

	archive, err := download(ctx, binURL)
	if err != nil {
		return "", err
	}
	if sumURL != "" {
		sums, err := download(ctx, sumURL)
		if err == nil {
			if want := checksumFor(sums, asset); want != "" {
				got := sha256.Sum256(archive)
				if hex.EncodeToString(got[:]) != want {
					return "", fmt.Errorf("checksum mismatch for %s — aborting", asset)
				}
			}
		}
	}

	bin, err := extractBinary(archive)
	if err != nil {
		return "", err
	}
	if err := replaceSelf(bin); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

// replaceSelf writes bin next to the current executable and atomically renames
// it over the original (same directory keeps the rename atomic).
func replaceSelf(bin []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".tmax-update-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w (try sudo, or reinstall via the install script)", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmpName, exe); err != nil {
		return fmt.Errorf("cannot replace %s: %w (try sudo, or reinstall via the install script)", exe, err)
	}
	return nil
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

// extractBinary pulls the "tmax" file out of a gzipped tarball.
func extractBinary(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(hdr.Name) == "tmax" && hdr.Typeflag == tar.TypeReg {
			return io.ReadAll(io.LimitReader(tr, 64<<20))
		}
	}
	return nil, errors.New("tmax binary not found in release archive")
}

// checksumFor returns the hex sha256 recorded for name in a goreleaser-style
// "<sha256>  <file>" checksums file.
func checksumFor(sums []byte, name string) string {
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[1] == name {
			return f[0]
		}
	}
	return ""
}

// ---- version comparison + state cache ----

// isNewer reports whether latest is a higher semantic version than current.
// A "dev"/unparseable current counts as older than any real release.
func isNewer(current, latest string) bool {
	if latest == "" {
		return false
	}
	lv, ok := parseSemver(latest)
	if !ok {
		return false
	}
	cv, ok := parseSemver(current)
	if !ok {
		return true // dev build: any tagged release is "newer"
	}
	for i := 0; i < 3; i++ {
		if lv[i] != cv[i] {
			return lv[i] > cv[i]
		}
	}
	return false
}

func parseSemver(s string) ([3]int, bool) {
	var v [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i] // drop pre-release / build metadata
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 || s == "" {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

type state struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

func statePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "tmax", "update-check.json")
}

func loadState() state {
	var st state
	p := statePath()
	if p == "" {
		return st
	}
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	return st
}

func saveState(st state) {
	p := statePath()
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	if b, err := json.Marshal(st); err == nil {
		_ = os.WriteFile(p, b, 0o644)
	}
}
