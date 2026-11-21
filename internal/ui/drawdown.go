package ui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"math"
	"portifolio/internal/domain"
	"strings"
)

// Preserve the deepest daily drawdown in each interval. Computing drawdown
// before grouping keeps peaks and troughs that period-end prices would hide.
func aggregateDrawdown(points []domain.Point, grain string) []domain.Point {
	var out []domain.Point
	for _, p := range points {
		start := candlePeriodStart(p.Date, grain)
		if len(out) == 0 || !candlePeriodStart(out[len(out)-1].Date, grain).Equal(start) {
			out = append(out, p)
		} else {
			last := &out[len(out)-1]
			last.Value = min(last.Value, p.Value)
			last.Date = p.Date
		}
	}
	return out
}
func (m Model) drawdownCard(curve []domain.Point, width int) string {
	plotWidth := max(8, min(54, width-28))
	all := aggregateDrawdown(drawdownSeries(curve), m.grain)
	visible := pointWindow(all, max(1, plotWidth/2), m.chartOffsets[2])
	heading := accent.Render("DRAWDOWN / HISTORY · " + strings.ToUpper(m.grain))
	body := heading + "\n" + muted.Render("Worst daily decline from prior peak in each period (%)") + "\n\n" + renderDrawdown(visible, plotWidth, 7)
	body += "\n" + muted.Render("z interval · ←/→ move one period · 0% = previous peak")
	return m.analysisCardStyle(width, 2).Render(body)
}
func renderDrawdown(points []domain.Point, width, height int) string {
	if len(points) == 0 {
		return muted.Render("No drawdown history available")
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
	fill := lipgloss.NewStyle().Foreground(lipgloss.Color("#733b4b"))
	edge := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6375"))
	var b strings.Builder
	for row := 0; row < height; row++ {
		b.WriteString(muted.Render(fmt.Sprintf("%7.2f%% │", low*float64(row)/float64(height-1))))
		for _, depth := range depths {
			cell := "  "
			if row < depth {
				cell = fill.Render("░░")
			} else if row == depth {
				cell = edge.Render("━━")
			}
			b.WriteString(cell)
		}
		b.WriteByte('\n')
	}
	b.WriteString(muted.Render("         └"+strings.Repeat("─", len(points)*2)) + "\n")
	// Sparse full dates, aligned with the same two-column periods as the plot.
	axis := []rune(strings.Repeat(" ", max(10, len(points)*2)))
	next := 0
	for i, p := range points {
		label := p.Date.Format("2006-01-02")
		pos := i * 2
		if i == len(points)-1 {
			pos = max(0, len(axis)-len(label))
		}
		if pos < next || pos+len(label) > len(axis) {
			continue
		}
		copy(axis[pos:], []rune(label))
		next = pos + len(label) + 2
	}
	b.WriteString(muted.Render("          " + strings.TrimRight(string(axis), " ")))
	return b.String()
}
