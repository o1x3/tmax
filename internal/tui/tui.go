// Package tui provides the interactive Bubble Tea front-end for tmax. It
// reuses the same lipgloss card renderer as the static output, but lets you
// flip between harnesses, tabs and time ranges live.
package tui

import (
	"time"

	"github.com/o1x3/tmax/internal/core"
	"github.com/o1x3/tmax/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	harnesses = []string{core.Combined, core.Claude, core.Codex, core.Pi}
	ranges    = []string{core.RangeAll, core.Range30d, core.Range7d}
	tabs      = []string{ui.TabOverview, ui.TabModels}
)

type model struct {
	aggs    map[string]*core.Aggregate
	hi, ri  int // harness / range index
	tab     int
	now     time.Time
	w, h    int
	hintCol lipgloss.Style
}

// New builds the interactive model with the given start harness/range/tab.
func New(harness, rng, tab string) model {
	m := model{
		aggs: map[string]*core.Aggregate{},
		now:  time.Now(),
	}
	for _, h := range harnesses {
		m.aggs[h] = core.Load(h)
	}
	m.hi = indexOf(harnesses, harness, 0)
	m.ri = indexOf(ranges, rng, 0)
	m.tab = indexOf(tabs, tab, 0)
	m.hintCol = lipgloss.NewStyle().Foreground(lipgloss.Color("#565668"))
	return m
}

func indexOf(s []string, v string, def int) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return def
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "tab", "m":
			m.tab = (m.tab + 1) % len(tabs)
		case "shift+tab":
			m.tab = (m.tab - 1 + len(tabs)) % len(tabs)
		case "right", "l":
			m.hi = (m.hi + 1) % len(harnesses)
		case "left", "h":
			m.hi = (m.hi - 1 + len(harnesses)) % len(harnesses)
		case "1":
			m.ri = 0
		case "2":
			m.ri = 1
		case "3":
			m.ri = 2
		case "r":
			m.ri = (m.ri + 1) % len(ranges)
		}
	}
	return m, nil
}

func (m model) View() string {
	agg := m.aggs[harnesses[m.hi]]
	s := core.Summarize(agg, ranges[m.ri], m.now)
	card := ui.RenderCard(s, tabs[m.tab])

	hint := m.hintCol.Render("←/→ harness · tab models · 1/2/3 range · q quit")
	body := lipgloss.JoinVertical(lipgloss.Center, card, "", hint)

	if m.w > 0 && m.h > 0 {
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, body)
	}
	return body
}

// Run starts the interactive program.
func Run(harness, rng, tab string) error {
	p := tea.NewProgram(New(harness, rng, tab), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
