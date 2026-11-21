package chart

import (
	"fmt"
	"math"
	"strings"
	"time"

	"portifolio/core"
	"portifolio/ui/theme"
)

func Line(points []core.Point, width, height int) string {
	if len(points) < 2 {
		return theme.Muted.Render("No price history available")
	}
	width = max(20, width)
	height = max(4, height)
	lo, hi := points[0].Value, points[0].Value
	for _, p := range points {
		lo = min(lo, p.Value)
		hi = max(hi, p.Value)
	}
	if hi == lo {
		hi++
	}
	grid := make([][]rune, height)
	for i := range grid {
		grid[i] = []rune(strings.Repeat(" ", width))
	}
	for x := 0; x < width; x++ {
		v := points[x*(len(points)-1)/(width-1)].Value
		y := height - 1 - int((v-lo)/(hi-lo)*float64(height-1))
		grid[y][x] = '•'
	}
	var b strings.Builder
	for _, row := range grid {
		b.WriteString(string(row) + "\n")
	}
	b.WriteString(points[0].Date.Format("2006-01-02") + " → " + points[len(points)-1].Date.Format("2006-01-02"))
	return b.String()
}

func Drawdown(points []core.Point, width, height int) string {
	if len(points) == 0 {
		return theme.Muted.Render("No drawdown history available")
	}
	low := 0.0
	for _, p := range points {
		low = min(low, p.Value)
	}
	if low == 0 {
		low = -1
	}
	depths := make([]int, len(points))
	for i, p := range points {
		depths[i] = int(math.Round(p.Value / low * float64(height-1)))
	}
	var b strings.Builder
	for row := 0; row < height; row++ {
		val := low * float64(row) / float64(height-1)
		if val == 0 {
			val = 0
		}
		b.WriteString(theme.Muted.Render(fmt.Sprintf("%7.2f%% │", val)))
		for _, depth := range depths {
			cell := "  "
			if row < depth {
				cell = theme.LossFill.Render("░░")
			} else if row == depth {
				cell = theme.Loss.Render("━━")
			}
			b.WriteString(cell)
		}
		b.WriteByte('\n')
	}
	b.WriteString(theme.Muted.Render("         └"+strings.Repeat("─", len(points)*2)) + "\n")
	dates := make([]time.Time, len(points))
	for i, p := range points {
		dates[i] = p.Date
	}
	b.WriteString(DateAxis(dates, "          ", 10, false))
	return b.String()
}

func DriftHistory(points []core.Point, width, height int) string {
	if len(points) == 0 {
		return theme.Muted.Render("No drift history available")
	}
	width = max(18, width)
	height = max(3, height)
	maxDrift := 0.0
	for _, point := range points {
		maxDrift = max(maxDrift, point.Value)
	}
	if maxDrift <= 0 {
		maxDrift = 1
	}
	columns := max(8, width-9)
	var b strings.Builder
	for row := 0; row < height; row++ {
		label := maxDrift * float64(height-1-row) / float64(height-1)
		b.WriteString(theme.Muted.Render(fmt.Sprintf("%5.1f%% │", label)))
		for x := 0; x < columns; x++ {
			index := x * len(points) / columns
			value := points[index].Value / maxDrift * float64(height-1)
			if float64(height-1-row) <= value+0.001 {
				b.WriteString(theme.DriftBar.Render("█"))
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	b.WriteString(theme.Muted.Render("       └"+strings.Repeat("─", columns)) + "\n")
	b.WriteString(theme.Muted.Render(fmt.Sprintf("       %s → %s", points[0].Date.Format("2006-01-02"), points[len(points)-1].Date.Format("2006-01-02"))))
	return b.String()
}
