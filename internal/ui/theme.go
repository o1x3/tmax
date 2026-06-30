// Package ui renders the tmax dashboard card with lipgloss.
package ui

import (
	"github.com/o1x3/tmax/internal/core"

	"github.com/charmbracelet/lipgloss"
)

// Theme is a pastel colour scheme. Each harness gets its own accent + heatmap
// ramp so a glance tells you which harness you're looking at.
type Theme struct {
	Name   string
	Accent lipgloss.Color    // pill + headline accent
	Ramp   [5]lipgloss.Color // heatmap intensity: [0]=empty .. [4]=hottest
}

// shared chrome colours (dark card, pastel text)
var (
	cardBG     = lipgloss.Color("#15151c")
	border     = lipgloss.Color("#2c2c39")
	boxBG      = lipgloss.Color("#1d1d27")
	pillText   = lipgloss.Color("#15151c")
	title      = lipgloss.Color("#e9e9f2")
	label      = lipgloss.Color("#7a7a8c")
	value      = lipgloss.Color("#f2f2f7")
	muted      = lipgloss.Color("#565668")
	segInactBG = lipgloss.Color("#23232f")
	emptyCell  = lipgloss.Color("#23232e")
)

// ThemeFor returns the palette for a harness key.
func ThemeFor(harness string) Theme {
	switch harness {
	case core.Claude:
		// warm peach / coral — Claude's house colour, softened
		return Theme{
			Name:   "claude",
			Accent: "#f0b48a",
			Ramp: [5]lipgloss.Color{
				emptyCell, "#5c4433", "#9c6b4a", "#d99e76", "#ffcea8",
			},
		}
	case core.Codex:
		// mint / sea green
		return Theme{
			Name:   "codex",
			Accent: "#86e0b3",
			Ramp: [5]lipgloss.Color{
				emptyCell, "#2f5040", "#4e8467", "#7fc2a0", "#bdf3d8",
			},
		}
	case core.Pi:
		// lavender / periwinkle
		return Theme{
			Name:   "pi",
			Accent: "#b9a6ec",
			Ramp: [5]lipgloss.Color{
				emptyCell, "#473a5e", "#6f5a99", "#a48fd0", "#dccbff",
			},
		}
	default:
		// pastel blue — matches the reference
		return Theme{
			Name:   "all",
			Accent: "#9cc0f5",
			Ramp: [5]lipgloss.Color{
				emptyCell, "#34415c", "#52719f", "#86acde", "#c1dbff",
			},
		}
	}
}

// levelIndex maps a token count to a ramp index 0..4 (0 reserved for empty days).
func (t Theme) levelIndex(v, max int64) int {
	if v <= 0 {
		return 0
	}
	if max <= 0 {
		return 2
	}
	// log-ish bucketing so a few huge days don't wash everything out
	ratio := float64(v) / float64(max)
	switch {
	case ratio >= 0.6:
		return 4
	case ratio >= 0.3:
		return 3
	case ratio >= 0.1:
		return 2
	default:
		return 1
	}
}

// level maps a token count to its pastel ramp colour.
func (t Theme) level(v, max int64) lipgloss.Color { return t.Ramp[t.levelIndex(v, max)] }

// shadeGlyphs renders each ramp level as a 2-cell printable block, used when
// colour is unavailable (piped / ANSI stripped) so the heatmap density still
// reads structurally rather than collapsing to blank space.
var shadeGlyphs = [5]string{"  ", "░░", "▒▒", "▓▓", "██"}

// neofetch-style palette swatches: two rows of eight pastel hues (normal +
// bright). Decorative, constant across harnesses — every harness accent is
// itself present in the spread, so the card always shows its own colour.
var (
	swatchNormal = [8]lipgloss.Color{
		"#e8a0ad", "#f0b48a", "#ecd9a0", "#86e0b3", "#9fd8d8", "#9cc0f5", "#b9a6ec", "#e3a8d4",
	}
	swatchBright = [8]lipgloss.Color{
		"#f2c2cb", "#f8d2b6", "#f5e8c6", "#b6efd0", "#c8eaea", "#c4dafa", "#d6caf6", "#f0c8e6",
	}
)
