package ui

import (
	"math"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/ui/theme"
)

type weightState struct {
	symbol string
	input  string
}

func (m *Model) updateWeight(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "esc":
		m.page = holdingsPage
		return nil
	case "enter":
		value, err := strconv.ParseFloat(m.weight.input, 64)
		idx := m.positionIndex(m.weight.symbol)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 || idx < 0 {
			m.status = "Invalid allocation; press Esc to return"
			return nil
		}
		c := m.cfg.Clone()
		c.Portfolio.Positions[idx].Allocation = value / 100
		if err := m.kernel.SaveConfig(m.cfgPath, c); err != nil {
			m.status = err.Error()
			return nil
		}
		m.cfg = c
		m.status = "Saved"
		m.page = holdingsPage
		return nil
	case "backspace":
		m.weight.input = trimLast(m.weight.input)
	default:
		if key.Type == tea.KeyRunes {
			m.weight.input += string(key.Runes)
		}
	}
	return nil
}

func (m Model) viewWeight() string {
	return "TARGET ALLOCATION (%)\n\n" + theme.Highlight.Render(m.weight.input+"▏") + "\n\nEnter save · Esc cancel\n" + m.status
}
