package ui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)

func coloredAmount(value float64, currency string) string {
	color := "#929ba5"
	if value > 0 {
		color = "#35c98b"
	} else if value < 0 {
		color = "#ff6375"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(fmt.Sprintf("%+.2f %s", value, currency))
}

func coloredPercent(value float64) string {
	color := "#929ba5"
	if value > 0 {
		color = "#35c98b"
	} else if value < 0 {
		color = "#ff6375"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(fmt.Sprintf("%+.2f%%", value))
}

// analysisCardStyle reserves two distinct states for Analysis cards:
// yellow rounded border for selection, cyan double border for activation.
func (m Model) analysisCardStyle(width, index int) lipgloss.Style {
	if m.analysisCard != index {
		return cardStyle(width, false)
	}
	if m.analysisActive {
		return lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#55d6be")).
			Bold(true).
			Padding(0, 2).
			Width(max(18, min(76, width-8)))
	}
	return cardStyle(width, true)
}
