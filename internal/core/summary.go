package core

import (
	"fmt"
	"time"
)

// Time ranges.
const (
	RangeAll = "all"
	Range30d = "30d"
	Range7d  = "7d"
)

// RangeDays returns the number of days a range covers, or 0 for "all".
func RangeDays(r string) int {
	switch r {
	case Range7d:
		return 7
	case Range30d:
		return 30
	default:
		return 0
	}
}

// HobbitTokens is roughly the token count of The Hobbit (~95k words).
const HobbitTokens = 123528

// Summary is everything the UI needs for one harness + range.
type Summary struct {
	Harness       string
	Range         string
	Sessions      int
	Messages      int
	TotalTokens   int64
	InputTokens   int64
	OutputTokens  int64
	CacheTokens   int64
	ActiveDays    int
	CurrentStreak int
	LongestStreak int
	PeakHour      int
	FavModel      string
	Models        []ModelStat
	Heatmap       Heatmap
	HobbitFactor  float64
}

// Heatmap is a GitHub-style contribution grid: 7 rows (Sun..Sat) by N week
// columns. Values are token counts; -1 means the cell is out of range.
type Heatmap struct {
	Cells    [7][]int64 // [row][col]
	Weeks    int
	Max      int64
	FirstDay time.Time // civil date at row 0 (Sunday) of the leftmost column; drives month labels
}

// Summarize derives a Summary for the given range relative to now.
func Summarize(a *Aggregate, rng string, now time.Time) Summary {
	days := RangeDays(rng)

	// scalar stats are computed over the selected window
	s := Summary{
		Harness:  a.Harness,
		Range:    rng,
		Sessions: a.Sessions,
	}

	// window bounds (inclusive). zero cutoff => everything up to today.
	today := civil(now)
	var cutoff time.Time
	if days > 0 {
		cutoff = today.AddDate(0, 0, -(days - 1))
	}

	inWindow := func(dayStr string) bool {
		d, err := time.ParseInLocation("2006-01-02", dayStr, time.Local)
		if err != nil || d.After(today) {
			return false // never count future-dated days
		}
		return days == 0 || !d.Before(cutoff)
	}

	for day, n := range a.ByDayMsgs {
		if inWindow(day) {
			s.Messages += n
			if n > 0 {
				s.ActiveDays++
			}
		}
	}
	for day, tok := range a.ByDayTokens {
		if inWindow(day) {
			s.TotalTokens += tok
		}
	}

	// Peak hour, favourite model and the model breakdown are all windowed via
	// the per-day structures, so they match the selected range.
	s.PeakHour = a.PeakHourIn(inWindow)
	s.Models = a.TopModelsIn(inWindow)
	s.FavModel = a.FavoriteModelIn(inWindow)

	// For all-time we report the true ledger totals (exact cache split); for
	// windowed ranges we only have per-day token sums, so we approximate the
	// input/output/cache breakdown proportionally.
	if days == 0 {
		s.InputTokens = a.InputTokens
		s.OutputTokens = a.OutputTokens
		s.CacheTokens = a.CacheReadTokens + a.CacheWriteTokens
		s.TotalTokens = a.TotalTokens() // authoritative ledger total for all-time
	} else if total := a.TotalTokens(); total > 0 {
		f := float64(s.TotalTokens) / float64(total)
		s.InputTokens = int64(float64(a.InputTokens) * f)
		s.OutputTokens = int64(float64(a.OutputTokens) * f)
		s.CacheTokens = int64(float64(a.CacheReadTokens+a.CacheWriteTokens) * f)
	}

	s.CurrentStreak, s.LongestStreak = a.Streaks(now)
	if s.TotalTokens > 0 {
		s.HobbitFactor = float64(s.TotalTokens) / float64(HobbitTokens)
	}
	s.Heatmap = buildHeatmap(a, rng, now)
	return s
}

// buildHeatmap lays out the contribution grid. The newest week is the last
// column; today sits at its weekday row.
func buildHeatmap(a *Aggregate, rng string, now time.Time) Heatmap {
	// The heatmap always spans the same number of weeks so it fills the card
	// at every range; the range only governs which cells light up. 22 weeks
	// leaves room for the 4-col weekday gutter (Mon/Wed/Fri) within innerWidth.
	const weeks = 22

	today := civil(now)
	// Sunday of the current week.
	weekStart := today.AddDate(0, 0, -int(today.Weekday()))
	first := weekStart.AddDate(0, 0, -7*(weeks-1))

	// For a windowed range we blank days before the window. For "all" we
	// leave rangeStart zero so the whole grid shows as squares (empty days
	// included), GitHub-style, rather than a ragged blank left edge.
	var rangeStart time.Time
	if d := RangeDays(rng); d > 0 {
		rangeStart = today.AddDate(0, 0, -(d - 1))
	}

	h := Heatmap{Weeks: weeks, FirstDay: first}
	for r := 0; r < 7; r++ {
		h.Cells[r] = make([]int64, weeks)
	}
	for col := 0; col < weeks; col++ {
		for row := 0; row < 7; row++ {
			d := first.AddDate(0, 0, col*7+row)
			switch {
			case d.After(today):
				h.Cells[row][col] = -1 // future → ragged blank corner
			case !rangeStart.IsZero() && d.Before(rangeStart):
				h.Cells[row][col] = 0 // out of window → dim empty square
			default:
				v := a.ByDayTokens[d.Format("2006-01-02")]
				h.Cells[row][col] = v
				if v > h.Max {
					h.Max = v
				}
			}
		}
	}
	return h
}

// FormatTokens renders a token count as a compact human string (35.1M, 2.6B).
// Thresholds are set so a value that rounds up to 1.0 at the next unit is
// promoted (999,999 -> "1M", not "1000K").
func FormatTokens(n int64) string {
	abs := n
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs >= 999_950_000_000:
		return trimZero(float64(n)/1e12) + "T"
	case abs >= 999_950_000:
		return trimZero(float64(n)/1e9) + "B"
	case abs >= 999_950:
		return trimZero(float64(n)/1e6) + "M"
	case abs >= 1_000:
		return trimZero(float64(n)/1e3) + "K"
	default:
		return fmt.Sprintf("%d", n)
	}
}

func trimZero(f float64) string {
	s := fmt.Sprintf("%.1f", f)
	if len(s) > 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}

// FormatInt adds thousands separators (130,502).
func FormatInt(n int) string {
	s := fmt.Sprintf("%d", n)
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg, s = true, s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// FormatHour turns a 0-23 hour into "12 PM" style.
func FormatHour(h int) string {
	if h < 0 {
		return "—"
	}
	ampm := "AM"
	hh := h
	if h == 0 {
		hh = 12
	} else if h == 12 {
		ampm = "PM"
	} else if h > 12 {
		hh = h - 12
		ampm = "PM"
	}
	return fmt.Sprintf("%d %s", hh, ampm)
}

// HobbitLine renders the playful footer comparison.
func HobbitLine(factor float64) string {
	if factor <= 0 {
		return "No tokens recorded yet — go build something."
	}
	if factor < 1 {
		return fmt.Sprintf("You've used ~%.0f%% of the tokens in The Hobbit.", factor*100)
	}
	return fmt.Sprintf("You've used ~%s× more tokens than The Hobbit.", compactFactor(factor))
}

func compactFactor(f float64) string {
	if f >= 999.5 { // promotes 999.6 -> "1K" rather than "1000"
		return trimZero(f/1000) + "K"
	}
	return fmt.Sprintf("%.0f", f)
}
