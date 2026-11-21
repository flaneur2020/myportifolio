package ui

import (
	"fmt"
	"portifolio/internal/domain"
	"portifolio/internal/provider"
	"time"
)

// These are close-to-close model candles, not tradable intraday portfolio OHLC.
func portfolioCandles(curve []domain.Point) []provider.Bar {
	out := make([]provider.Bar, 0, len(curve))
	for i, point := range curve {
		open := point.Value
		if i > 0 {
			open = curve[i-1].Value
		}
		out = append(out, provider.Bar{Date: point.Date, Open: open, High: max(open, point.Value), Low: min(open, point.Value), Close: point.Value})
	}
	return out
}

// Pan over complete, aggregated periods. Keep the same number of visible
// candles at either boundary; clamp instead of shrinking or emptying the window.
func candleWindow(bars []provider.Bar, capacity, offset int) []provider.Bar {
	if len(bars) == 0 {
		return nil
	}
	capacity = max(1, min(capacity, len(bars)))
	offset = max(0, min(offset, len(bars)-capacity))
	end := len(bars) - offset
	return bars[end-capacity : end]
}
func (m Model) analysisCandles(curve []domain.Point, width int) string {
	all := aggregateCandles(portfolioCandles(curve), m.grain)
	capacity := max(1, (max(20, min(68, width-16))-12)/2)
	// Each key press moves by one candle while preserving the visible width.
	visible := candleWindow(all, capacity, m.offset)
	if len(visible) == 0 {
		return "No portfolio history available"
	}
	// Aggregated bars carry the final trading date of each period. Recover
	// the first daily observation belonging to the first visible period.
	coverageStart := visible[0].Date
	periodStart := candlePeriodStart(coverageStart, m.grain)
	for _, point := range curve {
		if !point.Date.Before(periodStart) && !point.Date.After(visible[0].Date) {
			coverageStart = point.Date
			break
		}
	}
	return m.analysisCardStyle(width, 1).Render(
		accent.Render("PORTFOLIO HISTORY · "+m.grain) + "\n\n" +
			renderCandles(visible, "day", max(20, min(68, width-16)), 12) + "\n" +
			muted.Render(fmt.Sprintf("Window: %s → %s", coverageStart.Format("2006-01-02"), visible[len(visible)-1].Date.Format("2006-01-02"))) + "\n" +
			muted.Render("←/→ move one candle · z day/week/month/quarter/year") + "\n" +
			muted.Render("Model candles use daily closes; wicks are not intraday highs/lows."))
}

func candlePeriodStart(date time.Time, grain string) time.Time {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	switch grain {
	case "week":
		return date.AddDate(0, 0, -(int(date.Weekday())+6)%7)
	case "month":
		return time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	case "quarter":
		return time.Date(date.Year(), time.Month((int(date.Month())-1)/3*3+1), 1, 0, 0, 0, 0, date.Location())
	case "year":
		return time.Date(date.Year(), 1, 1, 0, 0, 0, 0, date.Location())
	default:
		return date
	}
}
