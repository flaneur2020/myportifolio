package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"portifolio/internal/config"
	"portifolio/internal/ui/theme"
)

type setupState struct {
	marketIndex int
}

func (m *Model) updateSetup(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "up", "down", "left", "right":
		m.setup.marketIndex = 1 - m.setup.marketIndex
	case "enter":
		market := config.MarketUS
		if m.setup.marketIndex == 1 {
			market = config.MarketCN
		}
		c, _ := config.New(market)
		if err := config.Save(m.cfgPath, c); err != nil {
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
	choices := []string{"United States · USD · Yahoo / Stooq", "China A-shares · CNY · Eastmoney"}
	var b strings.Builder
	b.WriteString(theme.Accent.Render("WELCOME / CHOOSE A MARKET") + "\n\n")
	for i, c := range choices {
		style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
		if i == m.setup.marketIndex {
			style = style.BorderForeground(lipgloss.Color(theme.ColorHighlight))
			c = "▸ " + c
		}
		b.WriteString(style.Render(c) + "\n")
	}
	b.WriteString("\n↑/↓ choose · Enter open portfolio · q quit\n" + m.status)
	return b.String()
}
