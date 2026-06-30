package ui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/o1x3/tmax/internal/core"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	// deterministic colour output for width assertions
	lipgloss.SetColorProfile(termenv.TrueColor)
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func dispWidth(s string) int { return lipgloss.Width(ansi.ReplaceAllString(s, "")) }

func sampleSummary(tab string) core.Summary {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.Local)
	a := &core.Aggregate{
		Harness:     core.Combined,
		Sessions:    102,
		Messages:    16400,
		InputTokens: 5_000_000, OutputTokens: 2_000_000,
		CacheReadTokens: 28_000_000,
		ByDayTokens:     map[string]int64{"2026-06-29": 1_000_000, "2026-06-28": 500_000},
		ByDayMsgs:       map[string]int{"2026-06-29": 50, "2026-06-28": 30},
		ByDayHour:       map[string]*[24]int{"2026-06-29": {12: 50}, "2026-06-28": {9: 30}},
		ByDayModelTok: map[string]map[string]int64{
			"2026-06-29": {"claude-opus-4-8": 30_000_000, "gpt-5.4": 5_000_000},
		},
		ByDayModelMsg: map[string]map[string]int{
			"2026-06-29": {"claude-opus-4-8": 100, "gpt-5.4": 20},
		},
	}
	return core.Summarize(a, core.RangeAll, now)
}

func TestRenderCardOverview(t *testing.T) {
	out := RenderCard(sampleSummary(TabOverview), TabOverview)
	for _, want := range []string{"Overview", "Models", "sessions", "tokens", "peak hour", "fav model", "Contributions", "Less", "More", "Hobbit"} {
		if !strings.Contains(out, want) {
			t.Errorf("overview missing %q", want)
		}
	}
}

func TestRenderCardModels(t *testing.T) {
	out := RenderCard(sampleSummary(TabModels), TabModels)
	if !strings.Contains(out, "Token share by model") {
		t.Error("models view missing heading")
	}
	if !strings.Contains(out, "Opus 4.8") {
		t.Error("models view missing top model")
	}
}

// Every rendered line must be the same display width — guards against the
// wrapping / ragged-background bugs we hit during development.
func TestRenderCardUniformWidth(t *testing.T) {
	for _, tab := range []string{TabOverview, TabModels} {
		out := RenderCard(sampleSummary(tab), tab)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		w0 := dispWidth(lines[0])
		if w0 == 0 {
			t.Fatalf("%s: first line empty", tab)
		}
		for i, l := range lines {
			if w := dispWidth(l); w != w0 {
				t.Errorf("%s line %d width %d != %d:\n%q", tab, i, w, w0, l)
			}
		}
		if w0 > 80 {
			t.Errorf("%s width %d exceeds 80 cols", tab, w0)
		}
	}
}

// A month that spans several columns must always get a label, even when its
// first column collides with the previous month's label (regression: the label
// used to be dropped entirely ~23% of the time).
func TestMonthRowNoDroppedMonth(t *testing.T) {
	first := time.Date(2024, 9, 29, 0, 0, 0, 0, time.Local) // a Sunday
	h := core.Heatmap{Weeks: 22, FirstDay: first}
	for r := 0; r < 7; r++ {
		h.Cells[r] = make([]int64, 22)
	}
	row := ansi.ReplaceAllString(renderMonthRow(h, 22), "")
	for _, m := range []string{"Sep", "Oct", "Nov", "Dec", "Jan", "Feb"} {
		if !strings.Contains(row, m) {
			t.Errorf("month row dropped %q: %q", m, row)
		}
	}
}

// When colour is stripped (piped output) the heatmap falls back to shade
// glyphs and the swatches to █ blocks; the card must still be uniform width.
func TestRenderCardAsciiWidth(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	defer lipgloss.SetColorProfile(termenv.TrueColor)

	for _, tab := range []string{TabOverview, TabModels} {
		out := RenderCard(sampleSummary(tab), tab)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		w0 := dispWidth(lines[0])
		for i, l := range lines {
			if w := dispWidth(l); w != w0 {
				t.Errorf("ascii %s line %d width %d != %d:\n%q", tab, i, w, w0, l)
			}
		}
	}
}
