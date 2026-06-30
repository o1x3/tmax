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
	lipgloss.SetHasDarkBackground(true)
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

// The card paints no background, so lines are intentionally ragged — but none
// should exceed the terminal width budget (guards against wrapping / overflow).
func TestRenderCardMaxWidth(t *testing.T) {
	for _, tab := range []string{TabOverview, TabModels} {
		out := RenderCard(sampleSummary(tab), tab)
		for i, l := range strings.Split(out, "\n") {
			if w := dispWidth(l); w > 80 {
				t.Errorf("%s line %d width %d exceeds 80 cols:\n%q", tab, i, w, l)
			}
		}
	}
}

// The card must be fully transparent — no SGR sets a background colour, so it
// blends with the terminal (the whole point of the seamless redesign).
func TestRenderCardNoBackground(t *testing.T) {
	for _, tab := range []string{TabOverview, TabModels} {
		if hasBackgroundSGR(RenderCard(sampleSummary(tab), tab)) {
			t.Errorf("%s sets a background colour; output must be transparent", tab)
		}
	}
}

// hasBackgroundSGR parses SGR sequences and reports whether any sets a
// background, correctly skipping 38;2;r;g;b foreground channels (a channel of
// 48 must not be mistaken for the 48 background introducer).
func hasBackgroundSGR(s string) bool {
	for _, m := range ansi.FindAllStringSubmatch(s, -1) {
		ps := strings.Split(strings.Trim(m[0], "\x1b[m"), ";")
		for i := 0; i < len(ps); i++ {
			switch ps[i] {
			case "38", "48": // extended fg/bg: consume the colour spec
				bg := ps[i] == "48"
				if i+1 < len(ps) && ps[i+1] == "5" {
					i += 2
				} else if i+1 < len(ps) && ps[i+1] == "2" {
					i += 4
				}
				if bg {
					return true
				}
			case "40", "41", "42", "43", "44", "45", "46", "47", "49",
				"100", "101", "102", "103", "104", "105", "106", "107":
				return true
			}
		}
	}
	return false
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
// glyphs and the swatches to █ blocks; lines must still stay within budget.
func TestRenderCardAsciiWidth(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	defer lipgloss.SetColorProfile(termenv.TrueColor)

	for _, tab := range []string{TabOverview, TabModels} {
		out := RenderCard(sampleSummary(tab), tab)
		for i, l := range strings.Split(out, "\n") {
			if w := dispWidth(l); w > 80 {
				t.Errorf("ascii %s line %d width %d exceeds 80:\n%q", tab, i, w, l)
			}
		}
	}
}
