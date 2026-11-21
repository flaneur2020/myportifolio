package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"math"
	"portifolio/internal/config"
	"strconv"
	"strings"
)

func (m *Model) openPortfolioSettings() {
	p := m.cfg.Portfolios[0]
	m.settingsField = 0
	m.settingsRebalance = p.Rebalance
	m.settingsDate = p.StartDate
	capital := p.InitialCapital
	if capital <= 0 {
		capital = 10000
	}
	m.settingsCapital = strconv.FormatFloat(capital, 'f', -1, 64)
	m.status = ""
	m.page = settingsPage
}
func (m *Model) updatePortfolioSettings(key tea.KeyMsg) tea.Cmd {
	k := key.String()
	switch k {
	case "esc":
		m.page = analysisPage
		m.analysisActive = false
		m.status = ""
		return nil
	case "up", "shift+tab":
		m.settingsField = max(0, m.settingsField-1)
		return nil
	case "down", "tab":
		m.settingsField = min(3, m.settingsField+1)
		return nil
	case "left", "right":
		if m.settingsField == 0 {
			delta := 1
			if k == "left" {
				delta = -1
			}
			m.settingsRebalance = cycle([]string{"never", "monthly", "quarterly", "yearly"}, m.settingsRebalance, delta)
		}
		return nil
	case "enter":
		if m.settingsField < 3 {
			m.settingsField++
			return nil
		}
		return m.savePortfolioSettings()
	case "ctrl+s":
		return m.savePortfolioSettings()
	case "ctrl+u":
		if m.settingsField == 1 {
			m.settingsCapital = ""
		} else if m.settingsField == 2 {
			m.settingsDate = ""
		}
		return nil
	case "backspace":
		if m.settingsField == 1 {
			m.settingsCapital = trimLast(m.settingsCapital)
		} else if m.settingsField == 2 {
			m.settingsDate = trimLast(m.settingsDate)
		}
		return nil
	}
	if key.Type == tea.KeyRunes {
		if m.settingsField == 1 {
			m.settingsCapital += string(key.Runes)
		} else if m.settingsField == 2 {
			m.settingsDate += string(key.Runes)
		}
	}
	return nil
}
func (m *Model) savePortfolioSettings() tea.Cmd {
	capital, err := strconv.ParseFloat(strings.TrimSpace(m.settingsCapital), 64)
	if err != nil || math.IsNaN(capital) || math.IsInf(capital, 0) || capital <= 0 {
		m.status = "Initial capital must be greater than zero."
		m.settingsField = 1
		return nil
	}
	c := cloneConfig(m.cfg)
	p := &c.Portfolios[0]
	p.Rebalance = m.settingsRebalance
	p.InitialCapital = capital
	p.StartDate = strings.TrimSpace(m.settingsDate)
	if err := config.Save(m.cfgPath, c); err != nil {
		m.status = err.Error()
		return nil
	}
	reload := m.cfg.Portfolios[0].StartDate != p.StartDate
	m.cfg = c
	m.page = analysisPage
	m.analysisActive = false
	m.status = "Portfolio settings saved"
	if reload {
		m.offset = 0
		m.requested = map[string]bool{}
		return m.loadAll()
	}
	return nil
}
func (m Model) portfolioSettingsView(width int) string {
	values := []string{"‹ " + m.settingsRebalance + " ›", m.settingsCapital + " " + m.cfg.Portfolios[0].BaseCurrency, m.settingsDate, "Save changes"}
	labels := []string{"Rebalance", "Initial capital", "Start date", ""}
	var b strings.Builder
	b.WriteString(accent.Render("EDIT PORTFOLIO") + "\n\n")
	for i, value := range values {
		prefix := "  "
		if i == m.settingsField {
			prefix = "▸ "
			value = highlight.Render(value + " ▏")
		}
		b.WriteString(fmt.Sprintf("%s%-18s %s\n", prefix, labels[i], value))
	}
	b.WriteString("\n" + muted.Render("↑/↓ fields · ←/→ rebalance · Ctrl+U clear field\nEnter next/save · Ctrl+S save · Esc cancel"))
	if m.status != "" {
		b.WriteString("\n\n" + m.status)
	}
	return cardStyle(width, true).Render(b.String())
}
