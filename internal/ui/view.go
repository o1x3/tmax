package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/o1x3/tmax/internal/core"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Tabs.
const (
	TabOverview = "overview"
	TabModels   = "models"
)

const (
	innerWidth = 71 // content width inside the card padding
	hPad       = 2  // horizontal card padding
	logoW      = 36 // neofetch logo column (ANSI-Shadow wordmark is exactly 36)
	dividerW   = 3  // " │ " vertical rule between logo and info
	infoW      = innerWidth - logoW - dividerW // 32: the key·value info column
	gutterW    = 4  // weekday gutter ("Mon ") on the contribution graph
)

// logoArt is the "tmax" wordmark (pyfiglet "ANSI Shadow"). Every row is exactly
// logoW cells wide; it is recoloured per harness via the accent, never stored
// per-harness.
var logoArt = [6]string{
	"████████╗███╗   ███╗ █████╗ ██╗  ██╗",
	"╚══██╔══╝████╗ ████║██╔══██╗╚██╗██╔╝",
	"   ██║   ██╔████╔██║███████║ ╚███╔╝ ",
	"   ██║   ██║╚██╔╝██║██╔══██║ ██╔██╗ ",
	"   ██║   ██║ ╚═╝ ██║██║  ██║██╔╝ ██╗",
	"   ╚═╝   ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝",
}

// styled is the workhorse: a card-background style with the given foreground.
// Every visible run sets Background(cardBG) so trailing/leading fill never
// shows the terminal's default background (which would make the card ragged).
func styled(fg lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Background(cardBG).Foreground(fg)
}

func ascii() bool { return lipgloss.ColorProfile() == termenv.Ascii }

// RenderCard renders the full neofetch-style dashboard for a summary.
func RenderCard(s core.Summary, tab string) string {
	th := ThemeFor(s.Harness)

	controls := renderHeader(th, tab, s.Range)
	banner := renderBanner(th, s)

	var mid string
	if tab == TabModels {
		mid = renderModels(th, s)
	} else {
		mid = lipgloss.JoinVertical(lipgloss.Left,
			sectionTitle(th, s.Heatmap.Weeks),
			renderHeatmap(th, s.Heatmap),
		)
	}

	footer := renderFooter(th, s)
	swatches := renderSwatches()

	content := lipgloss.JoinVertical(lipgloss.Left,
		controls,
		"",
		banner,
		"",
		mid,
		"",
		footer,
		"",
		swatches,
	)

	card := lipgloss.NewStyle().
		Background(cardBG).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(cardBG).
		Width(innerWidth + 2*hPad). // content area == innerWidth; short lines fill with cardBG
		Padding(1, hPad)

	return card.Render(content)
}

// ---- header: tabs (left) + range segmented control (right) ----

func renderHeader(th Theme, tab, rng string) string {
	active := lipgloss.NewStyle().
		Background(th.Accent).Foreground(pillText).Bold(true).Padding(0, 1)
	inactive := lipgloss.NewStyle().
		Foreground(label).Background(cardBG).Padding(0, 2)

	// Literal brackets on the active tab so the selection still reads after
	// ANSI is stripped (the accent pill background vanishes when piped).
	overview := inactive.Render("Overview")
	models := inactive.Render("Models")
	if tab == TabModels {
		models = active.Render("[ Models ]")
	} else {
		overview = active.Render("[ Overview ]")
	}
	left := lipgloss.JoinHorizontal(lipgloss.Center, overview, " ", models)

	right := renderRange(rng)

	gap := innerWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	spacer := styled(cardBG).Render(strings.Repeat(" ", gap))
	return lipgloss.JoinHorizontal(lipgloss.Center, left, spacer, right)
}

func renderRange(rng string) string {
	active := lipgloss.NewStyle().
		Background(segInactBG).Foreground(value).Bold(true).Padding(0, 1)
	if ascii() {
		active = active.Underline(true) // distinguish the active range when piped
	}
	inactive := lipgloss.NewStyle().
		Background(cardBG).Foreground(muted).Padding(0, 1)

	seg := func(key, lbl string) string {
		if rng == key {
			return active.Render(lbl)
		}
		return inactive.Render(lbl)
	}
	return lipgloss.JoinHorizontal(lipgloss.Center,
		seg(core.RangeAll, "All"),
		seg(core.Range30d, "30d"),
		seg(core.Range7d, "7d"),
	)
}

// ---- neofetch banner: logo column | key·value info column ----

func renderBanner(th Theme, s core.Summary) string {
	host := hostUser()
	title := styled(th.Accent).Bold(true).Render(host) +
		styled(muted).Render("@") +
		styled(th.Accent).Bold(true).Render(th.Name)
	titleW := lipgloss.Width(host) + 1 + lipgloss.Width(th.Name)
	underline := styled(th.Accent).Render(strings.Repeat("─", titleW))

	info := []string{
		padBGTo(title, infoW),
		padBGTo(underline, infoW),
		leaderRow("sessions", core.FormatInt(s.Sessions), infoW),
		leaderRow("messages", core.FormatInt(s.Messages), infoW),
		leaderRow("tokens", core.FormatTokens(s.TotalTokens), infoW),
		leaderRow("active days", core.FormatInt(s.ActiveDays), infoW),
		leaderRow("streak", fmt.Sprintf("%dd / %dd", s.CurrentStreak, s.LongestStreak), infoW),
		leaderRow("peak hour", core.FormatHour(s.PeakHour), infoW),
		leaderRow("fav model", s.FavModel, infoW),
	}

	// Top-align the logo with the title; a full-height rule separates the two
	// panes so the info list reading on past the logo's last row looks intended.
	divider := styled(cardBG).Render(" ") +
		lipgloss.NewStyle().Background(cardBG).Foreground(border).Render("│") +
		styled(cardBG).Render(" ")
	rows := make([]string, len(info))
	for i := range info {
		var logo string
		if i < len(logoArt) {
			logo = styled(th.Accent).Render(padRight(logoArt[i], logoW))
		} else {
			logo = styled(cardBG).Render(strings.Repeat(" ", logoW))
		}
		rows[i] = padBG(logo + divider + info[i])
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// leaderRow renders a neofetch-style "key ···· value" line padded to colW, with
// computed dot leaders so values right-align. The value is truncated to fit.
func leaderRow(key, val string, colW int) string {
	kW := lipgloss.Width(key)
	maxVal := colW - kW - 3 // space + at least one dot + space
	if maxVal < 1 {
		maxVal = 1
	}
	val = truncate(val, maxVal)
	n := colW - kW - lipgloss.Width(val) - 2
	if n < 1 {
		n = 1
	}
	line := styled(label).Render(key) +
		styled(muted).Render(" "+strings.Repeat("·", n)+" ") +
		styled(value).Bold(true).Render(val)
	return padBGTo(line, colW)
}

// ---- contribution graph (GitHub-style hero) ----

func sectionTitle(th Theme, weeks int) string {
	return padBG(styled(th.Accent).Bold(true).Render("Contributions") +
		styled(muted).Render(fmt.Sprintf(" · last %d weeks", weeks)))
}

func renderHeatmap(th Theme, h core.Heatmap) string {
	if h.Weeks <= 0 {
		return ""
	}
	// Defensive: never let a caller-supplied week count overflow innerWidth.
	// A grid row is gutterW + 3*cols - 1 cells wide; production always passes 22.
	cols := h.Weeks
	if maxCols := (innerWidth - gutterW + 1) / 3; cols > maxCols {
		cols = maxCols
	}
	plain := ascii()
	gapCell := styled(cardBG).Render(" ")
	gut := [7]string{"    ", "Mon ", "    ", "Wed ", "    ", "Fri ", "    "}

	rows := make([]string, 0, 10)
	rows = append(rows, renderMonthRow(h, cols))
	for r := 0; r < 7; r++ {
		var sb strings.Builder
		sb.WriteString(styled(label).Render(gut[r]))
		for col := 0; col < cols; col++ {
			if col > 0 {
				sb.WriteString(gapCell)
			}
			sb.WriteString(heatCell(th, h.Cells[r][col], h.Max, plain))
		}
		rows = append(rows, padBG(sb.String()))
	}
	rows = append(rows, padBG(""), renderLegend(th, plain))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func heatCell(th Theme, v, max int64, plain bool) string {
	if v < 0 { // future → ragged blank corner
		return styled(cardBG).Render("  ")
	}
	if plain {
		return shadeGlyphs[th.levelIndex(v, max)]
	}
	return lipgloss.NewStyle().Background(th.level(v, max)).Render("  ")
}

// renderMonthRow places 3-letter month abbreviations above the column where
// each month begins. If that first column collides with the previous label, the
// month is deferred to its next column rather than dropped — `placed` advances
// only after a label is actually written.
func renderMonthRow(h core.Heatmap, cols int) string {
	buf := make([]rune, innerWidth)
	for i := range buf {
		buf[i] = ' '
	}
	lastEnd := -10
	placed := time.Month(0)
	for col := 0; col < cols; col++ {
		d := h.FirstDay.AddDate(0, 0, col*7)
		if d.Month() == placed {
			continue
		}
		x := gutterW + col*3
		if x < lastEnd+1 || x+3 > innerWidth {
			continue // collides; a later column of the same month will retry
		}
		ab := d.Format("Jan")
		for i := 0; i < len(ab); i++ {
			buf[x+i] = rune(ab[i])
		}
		placed = d.Month()
		lastEnd = x + 3
	}
	return styled(label).Render(string(buf))
}

func renderLegend(th Theme, plain bool) string {
	cell := func(i int) string {
		if plain {
			return shadeGlyphs[i]
		}
		return lipgloss.NewStyle().Background(th.Ramp[i]).Render("  ")
	}
	parts := []string{styled(muted).Render("Less ")}
	for i := 0; i < 5; i++ {
		if i > 0 {
			parts = append(parts, styled(cardBG).Render(" "))
		}
		parts = append(parts, cell(i))
	}
	parts = append(parts, styled(muted).Render(" More"))
	legend := strings.Join(parts, "")

	pad := innerWidth - lipgloss.Width(legend)
	if pad < 0 {
		pad = 0
	}
	return styled(cardBG).Render(strings.Repeat(" ", pad)) + legend
}

// ---- neofetch palette: two rows of pastel colour swatches ----

func renderSwatches() string {
	row := func(cols [8]lipgloss.Color) string {
		var sb strings.Builder
		for i, c := range cols {
			if i > 0 {
				sb.WriteString(styled(cardBG).Render(" "))
			}
			// Foreground (not Background) so the █ blocks survive ANSI stripping
			// as the palette silhouette when piped.
			sb.WriteString(lipgloss.NewStyle().Background(cardBG).Foreground(c).Render("██"))
		}
		return padBG(sb.String())
	}
	return lipgloss.JoinVertical(lipgloss.Left, row(swatchNormal), row(swatchBright))
}

// ---- models tab ----

func renderModels(th Theme, s core.Summary) string {
	if len(s.Models) == 0 {
		return padBG(styled(muted).Render("No model usage recorded."))
	}
	models := s.Models
	if len(models) > 8 {
		models = models[:8]
	}
	var max int64 = 1
	for _, m := range models {
		if m.Tokens > max {
			max = m.Tokens
		}
	}

	var total int64
	for _, m := range s.Models {
		total += m.Tokens
	}

	const nameW = 16
	const metaW = 16 // "  " + 7 tokens + "  " + 5 pct
	barW := innerWidth - nameW - metaW
	rows := make([]string, 0, len(models)+2)
	for _, m := range models {
		name := styled(value).Bold(true).Render(padRight(truncate(m.Name, nameW), nameW))

		filled := int(float64(m.Tokens) / float64(max) * float64(barW))
		if filled < 1 && m.Tokens > 0 {
			filled = 1
		}
		if filled > barW {
			filled = barW
		}
		barCol := th.level(m.Tokens, max)
		bar := lipgloss.NewStyle().Background(barCol).Render(strings.Repeat(" ", filled)) +
			lipgloss.NewStyle().Background(emptyCell).Render(strings.Repeat(" ", barW-filled))

		pct := "—"
		if total > 0 {
			p := float64(m.Tokens) / float64(total) * 100
			if p >= 100 {
				pct = "100%"
			} else {
				pct = fmt.Sprintf("%.1f%%", p)
			}
		}
		meta := styled(label).Render("  "+padLeft(truncate(core.FormatTokens(m.Tokens), 7), 7)+"  ") +
			styled(muted).Render(padLeft(pct, 5))

		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Center, name, bar, meta))
	}
	heading := padBG(styled(th.Accent).Bold(true).Render("Token share by model"))
	return lipgloss.JoinVertical(lipgloss.Left, heading, "", lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// ---- footer ----

func renderFooter(th Theme, s core.Summary) string {
	marker := styled(th.Accent).Render("› ")

	tag := strings.ToLower(s.Harness)
	if tag == core.Combined {
		tag = "all"
	}
	right := styled(th.Accent).Render(tag + " · " + rangeLabel(s.Range))

	hobMax := innerWidth - lipgloss.Width(marker) - lipgloss.Width(right) - 2
	hob := truncate(core.HobbitLine(s.HobbitFactor), hobMax)
	left := marker + styled(muted).Italic(true).Render(hob)

	gap := innerWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	spacer := styled(cardBG).Render(strings.Repeat(" ", gap))
	return padBG(lipgloss.JoinHorizontal(lipgloss.Center, left, spacer, right))
}

func rangeLabel(r string) string {
	switch r {
	case core.Range7d:
		return "last 7d"
	case core.Range30d:
		return "last 30d"
	default:
		return "all-time"
	}
}

// Hint renders a dim helper line shown under the card on a TTY.
func Hint(s string) string {
	return lipgloss.NewStyle().Foreground(muted).Italic(true).Render("  " + s)
}

// UpdateNotice renders the "a newer version is available" line shown under the
// card when the launch-time update check finds a release.
func UpdateNotice(latest string) string {
	accent := ThemeFor(core.Combined).Accent
	arrow := lipgloss.NewStyle().Foreground(accent).Bold(true).Render("  ↑ ")
	body := lipgloss.NewStyle().Foreground(muted).Render("tmax ") +
		lipgloss.NewStyle().Foreground(accent).Bold(true).Render(latest) +
		lipgloss.NewStyle().Foreground(muted).Render(" is available — run ") +
		lipgloss.NewStyle().Foreground(value).Render("tmax upgrade")
	return arrow + body
}

// ---- small helpers ----

func hostUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "you"
}

// padBG right-pads a styled line to innerWidth with the card background.
func padBG(s string) string { return padBGTo(s, innerWidth) }

// padBGTo right-pads a styled line to w cells with the card background, so
// trailing space after a style reset doesn't render as the terminal default.
func padBGTo(s string, w int) string {
	d := w - lipgloss.Width(s)
	if d <= 0 {
		return s
	}
	return s + styled(cardBG).Render(strings.Repeat(" ", d))
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	if w <= 1 {
		return s[:1]
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func padRight(s string, w int) string {
	d := w - lipgloss.Width(s)
	if d <= 0 {
		return s
	}
	return s + strings.Repeat(" ", d)
}

func padLeft(s string, w int) string {
	d := w - lipgloss.Width(s)
	if d <= 0 {
		return s
	}
	return strings.Repeat(" ", d) + s
}
