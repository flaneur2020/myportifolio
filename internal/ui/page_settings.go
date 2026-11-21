package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"portifolio/internal/ui/theme"
)

type settingsState struct {
	field     int
	rebalance string
	capital   string
	startDate string
}

func (m *Model) openSettings() {
	p := m.cfg.Portfolio
	capital := p.InitialCapital
	if capital <= 0 {
		capital = 10000
	}
	m.settings = settingsState{
		field:     0,
		rebalance: p.Rebalance,
		capital:   strconv.FormatFloat(capital, 'f', -1, 64),
		startDate: p.StartDate,
	}
	m.status = ""
	m.page = settingsPage
}

func (m *Model) updateSettings(key tea.KeyMsg) tea.Cmd {
	k := key.String()
	switch k {
	case "esc":
		m.page = analysisPage
		m.analysis.active = false
		m.status = ""
		return nil
	case "up", "shift+tab":
		m.settings.field = max(0, m.settings.field-1)
		return nil
	case "down", "tab":
		m.settings.field = min(3, m.settings.field+1)
		return nil
	case "left", "right":
		if m.settings.field == 0 {
			delta := 1
			if k == "left" {
				delta = -1
			}
			m.settings.rebalance = cycleString(config.RebalanceOptions, m.settings.rebalance, delta)
		}
		return nil
	case "enter":
		if m.settings.field < 3 {
			m.settings.field++
			return nil
		}
		return m.saveSettings()
	case "ctrl+s":
		return m.saveSettings()
	case "ctrl+u":
		if m.settings.field == 1 {
			m.settings.capital = ""
		} else if m.settings.field == 2 {
			m.settings.startDate = ""
		}
		return nil
	case "backspace":
		if m.settings.field == 1 {
			m.settings.capital = trimLast(m.settings.capital)
		} else if m.settings.field == 2 {
			m.settings.startDate = trimLast(m.settings.startDate)
		}
		return nil
	}
	if key.Type == tea.KeyRunes {
		if m.settings.field == 1 {
			m.settings.capital += string(key.Runes)
		} else if m.settings.field == 2 {
			m.settings.startDate += string(key.Runes)
		}
	}
	return nil
}

func (m *Model) saveSettings() tea.Cmd {
	capital, err := strconv.ParseFloat(strings.TrimSpace(m.settings.capital), 64)
	if err != nil || math.IsNaN(capital) || math.IsInf(capital, 0) || capital <= 0 {
		m.status = "Initial capital must be greater than zero."
		m.settings.field = 1
		return nil
	}
	c := m.cfg.Clone()
	p := &c.Portfolio
	p.Rebalance = m.settings.rebalance
	p.InitialCapital = capital
	p.StartDate = strings.TrimSpace(m.settings.startDate)
	if err := config.Save(m.cfgPath, c); err != nil {
		m.status = err.Error()
		return nil
	}
	reload := m.cfg.Portfolio.StartDate != p.StartDate
	m.cfg = c
	m.page = analysisPage
	m.analysis.active = false
	m.status = "Portfolio settings saved"
	if reload {
		m.analysis.setOffset("history", 0)
		m.requested = map[string]bool{}
		return m.loadAll()
	}
	return nil
}

func (m Model) viewSettings(width int) string {
	values := []string{
		"‹ " + m.settings.rebalance + " ›",
		m.settings.capital + " " + m.cfg.Portfolio.BaseCurrency,
		m.settings.startDate,
		"Save changes",
	}
	labels := []string{"Rebalance", "Initial capital", "Start date", ""}
	var b strings.Builder
	b.WriteString(theme.Accent.Render("EDIT PORTFOLIO") + "\n\n")
	for i, value := range values {
		prefix := "  "
		if i == m.settings.field {
			prefix = "▸ "
			value = theme.Highlight.Render(value + " ▏")
		}
		b.WriteString(fmt.Sprintf("%s%-18s %s\n", prefix, labels[i], value))
	}
	b.WriteString("\n" + theme.Muted.Render("↑/↓ fields · ←/→ rebalance · Ctrl+U clear field\nEnter next/save · Ctrl+S save · Esc cancel"))
	if m.status != "" {
		b.WriteString("\n\n" + m.status)
	}
	return theme.CardStyle(width, true).Render(b.String())
}
