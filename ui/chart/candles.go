package chart

import (
	"fmt"
	"math"
	"strings"
	"time"

	"portifolio/core"
	"portifolio/ui/theme"
)

func Candles(bars []core.Bar, grain core.Grain, width, height int) string {
	candles := core.AggregateCandles(bars, grain)
	if len(candles) == 0 {
		return theme.Muted.Render("No price history available")
	}
	width = max(20, width)
	height = max(4, height)
	capacity := max(1, (width-12)/2)
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
	var b strings.Builder
	for row := 0; row < height; row++ {
		price := high - float64(row)/(float64(height)-1)*(high-low)
		b.WriteString(theme.Muted.Render(fmt.Sprintf("%9.2f │", price)))
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
			style := theme.Gain
			if c.Close < c.Open {
				style = theme.Loss
			}
			b.WriteString(style.Render(glyph) + " ")
		}
		b.WriteByte('\n')
	}
	dates := make([]time.Time, len(candles))
	for i, c := range candles {
		dates[i] = c.Date
	}
	b.WriteString(DateAxis(dates, "           ", 0, false))
	return b.String()
}
