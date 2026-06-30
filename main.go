// tmax — a pastel terminal dashboard for your AI coding-harness token usage.
//
//	tmax                 combined stats from Claude Code, Codex and pi.dev
//	tmax claude          just Claude Code   (also: codex, pi)
//	tmax codex 7d        Codex, last 7 days (ranges: all, 30d, 7d)
//	tmax pi models       pi.dev, model breakdown
//	tmax -i              interactive mode (←/→ harness, tab, 1/2/3 range)
package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/o1x3/tmax/internal/core"
	"github.com/o1x3/tmax/internal/tui"
	"github.com/o1x3/tmax/internal/ui"
	"github.com/o1x3/tmax/internal/update"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
)

// version is overwritten at release time via -ldflags "-X main.version=...".
var version = "dev"

var ansiSeq = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type options struct {
	harness     string
	rng         string
	tab         string
	interactive bool
}

func parseArgs(args []string) (options, error) {
	o := options{harness: core.Combined, rng: core.RangeAll, tab: ui.TabOverview}
	for _, a := range args {
		switch a {
		case "-h", "--help", "help":
			return o, errHelp
		case "-i", "--interactive", "-t", "--tui", "tui":
			o.interactive = true
		case "claude", "cc", "claude-code":
			o.harness = core.Claude
		case "codex", "cx":
			o.harness = core.Codex
		case "pi", "pi.dev", "pidev":
			o.harness = core.Pi
		case "all", "combined", "everything":
			o.harness = core.Combined
		case "30d", "month", "30":
			o.rng = core.Range30d
		case "7d", "week", "7":
			o.rng = core.Range7d
		case "alltime", "lifetime":
			o.rng = core.RangeAll
		case "models", "model", "-m", "--models":
			o.tab = ui.TabModels
		case "overview", "-o", "--overview":
			o.tab = ui.TabOverview
		default:
			return o, fmt.Errorf("unknown argument %q (try: tmax --help)", a)
		}
	}
	return o, nil
}

var errHelp = fmt.Errorf("help")

const helpText = `tmax — token stats across your AI coding harnesses

USAGE
  tmax [harness] [range] [tab] [-i]

HARNESS   (default: all)
  claude            Claude Code        ~/.claude
  codex             OpenAI Codex       ~/.codex
  pi                pi.dev             ~/.pi
  all               every harness merged

RANGE     (default: all)
  all               lifetime
  30d               last 30 days
  7d                last 7 days

TAB       (default: overview)
  overview          headline stats + activity heatmap
  models            token share by model

FLAGS
  -i, --tui         interactive mode (←/→ harness · tab · 1/2/3 range · q quit)
  -h, --help        this help

COMMANDS
  upgrade           download and install the latest release
  version           print the installed version

EXAMPLES
  tmax              tmax codex 7d        tmax claude models     tmax pi -i`

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-v":
			fmt.Println("tmax " + version)
			return
		case "upgrade", "update", "self-update":
			runUpgrade()
			return
		}
	}

	forced := os.Getenv("TMAX_TRUECOLOR") != ""
	tty := isatty.IsTerminal(os.Stdout.Fd())

	// Colour handling: force 24-bit when asked (capture / under-reporting
	// terminals), strip colour when piped or redirected, otherwise let lipgloss
	// auto-detect (truecolor terminals get the full pastel palette).
	switch {
	case forced:
		lipgloss.SetColorProfile(termenv.TrueColor)
	case !tty:
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	o, err := parseArgs(args)
	if err == errHelp {
		fmt.Println(helpText)
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "tmax:", err)
		os.Exit(2)
	}

	if o.interactive {
		if err := tui.Run(o.harness, o.rng, o.tab); err != nil {
			fmt.Fprintln(os.Stderr, "tmax:", err)
			os.Exit(1)
		}
		return
	}

	agg := core.Load(o.harness)
	s := core.Summarize(agg, o.rng, time.Now())
	out := ui.RenderCard(s, o.tab)

	// Strip residual (empty) escape sequences for clean plaintext when piped.
	if !tty && !forced {
		out = ansiSeq.ReplaceAllString(out, "")
	}
	fmt.Println(out)

	// nudge toward the interactive view when on a real terminal
	if tty {
		fmt.Println(ui.Hint("run `tmax -i` for the interactive view"))
		// Throttled, best-effort: at most one network check per day; silent on
		// any failure. Skipped for dev builds and when TMAX_NO_UPDATE_CHECK is set.
		if version != "dev" {
			if latest, newer := update.Check(version); newer {
				fmt.Println(ui.UpdateNotice(latest))
			}
		}
	}
}

func runUpgrade() {
	fmt.Println("Checking for the latest tmax…")
	latest, err := update.SelfUpdate(version)
	switch {
	case errors.Is(err, update.ErrUpToDate):
		fmt.Printf("tmax %s is already the latest version.\n", version)
	case err != nil:
		fmt.Fprintln(os.Stderr, "tmax: upgrade failed:", err)
		os.Exit(1)
	default:
		fmt.Printf("Updated to tmax %s.\n", latest)
	}
}
