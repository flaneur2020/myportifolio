package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"math"
	"portifolio/internal/provider"
	"sort"
	"strings"
	"time"
)

func priceTickCmd() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg { return priceTick{} })
}
func (m Model) priceAnimation() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return accent.Render(frames[m.priceFrame%len(frames)]) + fmt.Sprintf(" Loading price history… (%d assets)", len(m.priceLoading))
}
func aggregateCandles(bars []provider.Bar, grain string) []provider.Bar {
	sorted := append([]provider.Bar(nil), bars...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })
	var out []provider.Bar
	previous := ""
	for _, bar := range sorted {
		if bar.Close <= 0 || math.IsNaN(bar.Close) || math.IsInf(bar.Close, 0) {
			continue
		}
		if bar.Open <= 0 {
			bar.Open = bar.Close
		}
		bar.High = max(bar.High, bar.Open, bar.Close)
		if bar.Low <= 0 {
			bar.Low = min(bar.Open, bar.Close)
		} else {
			bar.Low = min(bar.Low, bar.Open, bar.Close)
		}
		key := bar.Date.Format("2006-01-02")
		switch grain {
		case "week":
			y, w := bar.Date.ISOWeek()
			key = fmt.Sprintf("%d-W%02d", y, w)
		case "month":
			key = bar.Date.Format("2006-01")
		case "year":
			key = bar.Date.Format("2006")
		case "quarter":
			key = fmt.Sprintf("%d-Q%d", bar.Date.Year(), (int(bar.Date.Month())-1)/3+1)
		}
		if key != previous {
			out = append(out, bar)
			previous = key
		} else {
			p := &out[len(out)-1]
			p.High = max(p.High, bar.High)
			p.Low = min(p.Low, bar.Low)
			p.Close = bar.Close
			p.Date = bar.Date
		}
	}
	return out
}
func renderCandles(bars []provider.Bar, grain string, width, height int) string {
	candles := aggregateCandles(bars, grain)
	if len(candles) == 0 {
		return muted.Render("No price history available")
	}
	width = max(20, width)
	height = max(4, height)
	capacity := max(1, (width-12)/2)
	// Keep actual OHLC periods rather than inventing interpolated candles.
	if len(candles) > capacity {
		candles = candles[len(candles)-capacity:]
	}
	low, high := candles[0].Low, candles[0].High
	for _, c := range candles {
		low = min(low, c.Low)
		high = max(high, c.High)
	}
	if high == low {
		high = low + max(.01, low*.01)
	}
	y := func(v float64) int {
		return max(0, min(height-1, int(math.Round((high-v)/(high-low)*float64(height-1)))))
	}
	up := lipgloss.NewStyle().Foreground(lipgloss.Color("#35c98b"))
	down := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6375"))
	var b strings.Builder
	for row := 0; row < height; row++ {
		price := high - float64(row)/(float64(height)-1)*(high-low)
		b.WriteString(muted.Render(fmt.Sprintf("%9.2f │", price)))
		for _, c := range candles {
			glyph := " "
			top, bottom := y(max(c.Open, c.Close)), y(min(c.Open, c.Close))
			if row >= y(c.High) && row <= y(c.Low) {
				glyph = "│"
			}
			if row >= top && row <= bottom {
				glyph = "█"
				if top == bottom {
					glyph = "━"
				}
			}
			style := up
			if c.Close < c.Open {
				style = down
			}
			b.WriteString(style.Render(glyph) + " ")
		}
		b.WriteByte('\n')
	}
	b.WriteString(candleDateAxis(candles))
	return b.String()
}

// Place sparse date ticks at actual candle positions without overlapping labels.
func candleDateAxis(candles []provider.Bar) string {
	if len(candles) == 0 {
		return ""
	}
	columns := len(candles) * 2
	format := "2006-01-02"
	if columns < 10 {
		format = "01-02"
	}
	labelWidth := len(candles[0].Date.Format(format))
	if columns < labelWidth {
		return ""
	}
	axis := []rune(strings.Repeat(" ", columns))
	next := 0
	for i, candle := range candles {
		position := i * 2
		if i == len(candles)-1 {
			position = max(0, columns-labelWidth)
		}
		if position < next || position+labelWidth > columns {
			continue
		}
		copy(axis[position:], []rune(candle.Date.Format(format)))
		next = position + labelWidth + 2
	}
	return muted.Render("           " + strings.TrimRight(string(axis), " "))
}
