package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"portifolio/ui/theme"
)

type setupState struct {
	marketIndex int
}

func (m *Model) updateSetup(key tea.KeyMsg) tea.Cmd {
	markets := m.kernel.Markets()
	n := len(markets)
	switch key.String() {
	case "up", "k", "left":
		if n > 0 {
			m.setup.marketIndex = (m.setup.marketIndex - 1 + n) % n
		}
	case "down", "j", "right":
		if n > 0 {
			m.setup.marketIndex = (m.setup.marketIndex + 1) % n
		}
	case "enter":
		if n == 0 {
			m.status = "No market plugins registered."
			return nil
		}
		idx := max(0, min(m.setup.marketIndex, n-1))
		c, err := m.kernel.NewConfig(markets[idx].ID)
		if err != nil {
			m.status = err.Error()
			return nil
		}
		if err := m.kernel.SaveConfig(m.cfgPath, c); err != nil {
			m.status = err.Error()
			return nil
		}
		m.cfg = c
		m.page = holdingsPage
		return m.loadAll()
	case "q":
		return tea.Quit
	}
	return nil
}

func (m Model) viewSetup() string {
	markets := m.kernel.Markets()
	var b strings.Builder
	b.WriteString(theme.Accent.Render("WELCOME / CHOOSE A MARKET") + "\n\n")
	if len(markets) == 0 {
		b.WriteString(theme.Muted.Render("No market plugins loaded.") + "\n")
	}
	for i, market := range markets {
		label := market.Description
		if label == "" {
			label = market.Name + " · " + market.Currency
		}
		style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
		if i == m.setup.marketIndex {
			style = style.BorderForeground(lipgloss.Color(theme.ColorHighlight))
			label = "▸ " + label
		}
		b.WriteString(style.Render(label) + "\n")
	}
	b.WriteString("\n↑/↓ choose · Enter open portfolio · q quit\n" + m.status)
	return b.String()
}
