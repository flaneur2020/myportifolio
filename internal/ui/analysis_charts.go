package ui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"math"
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"strings"
)

func drawdownSeries(curve []domain.Point) []domain.Point {
	var out []domain.Point
	peak := 0.0
	for _, p := range curve {
		peak = max(peak, p.Value)
		if peak > 0 {
			out = append(out, domain.Point{Date: p.Date, Value: (p.Value/peak - 1) * 100})
		}
	}
	return out
}

// Returns compare successive period-end index values. The initial bucket uses
// the first model observation; it is explicitly a partial period.
func periodReturns(curve []domain.Point, grain string) []domain.Point {
	if len(curve) == 0 {
		return nil
	}
	ends := aggregateCandles(portfolioCandles(curve), grain)
	previous := curve[0].Value
	var out []domain.Point
	for _, end := range ends {
		if previous > 0 {
			out = append(out, domain.Point{Date: end.Date, Value: (end.Close/previous - 1) * 100})
		}
		previous = end.Close
	}
	return out
}
func (m Model) additionalAnalysisCharts(p config.Portfolio, curve []domain.Point, width int) string {
	chartWidth := max(12, min(48, width-26))
	drawdown := m.drawdownCard(curve, width)
	var weights strings.Builder
	weights.WriteString(accent.Render("TARGET ALLOCATION") + "\n")
	for _, pos := range p.Positions {
		if pos.Allocation <= 0 {
			continue
		}
		n := int(math.Round(pos.Allocation * float64(chartWidth)))
		weights.WriteString(fmt.Sprintf("%-10s %s %5.1f%%\n", pos.Symbol, accent.Render(strings.Repeat("━", n)), pos.Allocation*100))
	}
	weights.WriteString(muted.Render("Normalized target weights; not drifting market weights."))
	returns := periodReturns(curve, m.grain)
	capital := p.InitialCapital
	if capital <= 0 {
		capital = 10000
	}
	gains := periodGains(curve, m.grain, capital)
	gainByDate := make(map[string]float64, len(gains))
	for _, gain := range gains {
		gainByDate[gain.Date.Format("2006-01-02")] = gain.Value
	}
	returns = pointWindow(returns, 8, m.chartOffsets[4])
	scale := 0.0
	for _, r := range returns {
		scale = max(scale, math.Abs(r.Value))
	}
	var bars strings.Builder
	bars.WriteString(accent.Render("PERIOD RETURNS · "+strings.ToUpper(m.grain)) + "\n")
	// Keep calculations and paging chronological; reverse only presentation.
	for i := len(returns) - 1; i >= 0; i-- {
		r := returns[i]
		n := 0
		if scale > 0 {
			n = int(math.Round(math.Abs(r.Value) / scale * float64(chartWidth)))
		}
		bar := strings.Repeat("━", n)
		if r.Value < 0 {
			bar = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6375")).Render(bar)
		} else {
			bar = accent.Render(bar)
		}
		gain := gainByDate[r.Date.Format("2006-01-02")]
		bars.WriteString(fmt.Sprintf("%s  %s  %s\n  %s\n", r.Date.Format("2006-01-02"), coloredPercent(r.Value), coloredAmount(gain, p.BaseCurrency), bar))
	}
	bars.WriteString(muted.Render("z: day/week/month/quarter/year · ←/→ previous/next periods\nModel gain/loss · first model period may be partial."))
	return drawdown + "\n" + m.analysisCardStyle(width, 3).Render(weights.String()) + "\n" + m.analysisCardStyle(width, 4).Render(bars.String())
}

// Monetary returns use the change in model value, including prior gains/losses.
// For an index starting at 100, value = initial capital × index / 100.
func periodGains(curve []domain.Point, grain string, capital float64) []domain.Point {
	if len(curve) == 0 || curve[0].Value <= 0 {
		return nil
	}
	ends := aggregateCandles(portfolioCandles(curve), grain)
	previous := curve[0].Value
	var out []domain.Point
	for _, end := range ends {
		out = append(out, domain.Point{Date: end.Date, Value: capital * (end.Close - previous) / curve[0].Value})
		previous = end.Close
	}
	return out
}

// recentChange reports the latest model session's return and the monetary
// change implied by the configured starting capital.
func recentChange(curve []domain.Point, initialCapital float64) (percent, amount float64, ok bool) {
	if len(curve) < 2 || initialCapital <= 0 {
		return 0, 0, false
	}
	previous, latest := curve[len(curve)-2].Value, curve[len(curve)-1].Value
	if previous <= 0 {
		return 0, 0, false
	}
	percent = (latest/previous - 1) * 100
	amount = initialCapital * (latest - previous) / 100
	return percent, amount, true
}

// renderMetricRows keeps all values in one column. Padding is applied only to
// plain labels, since ANSI colour codes would otherwise distort fmt widths.
func renderMetricRows(rows [][2]string) string {
	const labelWidth = 17
	var b strings.Builder
	for _, row := range rows {
		if row[0] == "" {
			b.WriteByte('\n')
			continue
		}
		b.WriteString(muted.Render(fmt.Sprintf("%-*s", labelWidth, row[0])))
		b.WriteString("  " + row[1] + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func pointWindow(points []domain.Point, capacity, offset int) []domain.Point {
	if len(points) == 0 {
		return nil
	}
	capacity = min(capacity, len(points))
	offset = max(0, min(offset, len(points)-capacity))
	end := len(points) - offset
	return points[end-capacity : end]
}
