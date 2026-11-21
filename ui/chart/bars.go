package chart

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"portifolio/core"
	"portifolio/ui/theme"
)

func ReturnBars(points []core.Point, plotWidth, height int, capital float64, currency string) string {
	if len(points) == 0 {
		return theme.Muted.Render("No period returns available")
	}
	plotWidth = max(8, plotWidth)
	height = max(3, height)
	zeroRow := height / 2
	maxAbs := 0.0
	for _, point := range points {
		maxAbs = max(maxAbs, math.Abs(point.Value))
	}
	scalePercent := maxAbs
	if scalePercent == 0 {
		scalePercent = 1
	}
	targets := make([]int, len(points))
	for i, point := range points {
		targets[i] = zeroRow - int(math.Round(point.Value/scalePercent*float64(zeroRow)))
		targets[i] = max(0, min(height-1, targets[i]))
	}
	const labelWidth = 11
	var b strings.Builder
	for row := 0; row < height; row++ {
		money := capital * maxAbs / 100
		value := money*float64(height-1-row)/float64(height-1)*2 - money
		label := theme.FormatCompactMoney(value, currency)
		b.WriteString(theme.Muted.Render(fmt.Sprintf("%9s", label) + " │"))
		for i, point := range points {
			cell := "  "
			if row == zeroRow {
				cell = theme.Muted.Render("──")
			} else if point.Value > 0 && row >= targets[i] && row < zeroRow {
				if row == targets[i] {
					cell = theme.Gain.Render("━━")
				} else {
					cell = theme.GainFill.Render("░░")
				}
			} else if point.Value < 0 && row > zeroRow && row <= targets[i] {
				if row == targets[i] {
					cell = theme.Loss.Render("━━")
				} else {
					cell = theme.LossFill.Render("░░")
				}
			}
			b.WriteString(cell)
		}
		b.WriteByte('\n')
	}
	b.WriteString(theme.Muted.Render(strings.Repeat(" ", labelWidth)+"└"+strings.Repeat("─", len(points)*2)) + "\n")
	dates := make([]time.Time, len(points))
	for i, p := range points {
		dates[i] = p.Date
	}
	b.WriteString(DateAxis(dates, strings.Repeat(" ", labelWidth+1), 10, true))
	return b.String()
}

func StackedWeights(weights []float64, width int) string {
	if width <= 0 || len(weights) == 0 {
		return ""
	}
	sizes := make([]int, len(weights))
	used := 0
	for i, weight := range weights {
		if weight <= 0 {
			continue
		}
		sizes[i] = max(1, int(math.Round(weight*float64(width))))
		used += sizes[i]
	}
	for used > width {
		largest := 0
		for i := range sizes {
			if sizes[i] > sizes[largest] {
				largest = i
			}
		}
		if sizes[largest] <= 1 {
			break
		}
		sizes[largest]--
		used--
	}
	for used < width {
		largest := 0
		for i := range weights {
			if weights[i] > weights[largest] {
				largest = i
			}
		}
		sizes[largest]++
		used++
	}
	var b strings.Builder
	for i, size := range sizes {
		if size <= 0 {
			continue
		}
		b.WriteString(lipgloss.NewStyle().Foreground(theme.DriftColor(i)).Render(strings.Repeat("█", size)))
	}
	return b.String()
}
