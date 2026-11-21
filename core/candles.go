package core

import (
	"math"
	"sort"
)

func AggregateCandles(bars []Bar, grain Grain) []Bar {
	sorted := append([]Bar(nil), bars...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })
	var out []Bar
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
		key := PeriodKey(bar.Date, grain)
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

// PortfolioCandles builds close-to-close model candles, not tradable intraday OHLC.
func PortfolioCandles(curve []Point) []Bar {
	out := make([]Bar, 0, len(curve))
	for i, point := range curve {
		open := point.Value
		if i > 0 {
			open = curve[i-1].Value
		}
		out = append(out, Bar{
			Date:  point.Date,
			Open:  open,
			High:  max(open, point.Value),
			Low:   min(open, point.Value),
			Close: point.Value,
		})
	}
	return out
}

// CandleWindow pans over complete, aggregated periods without shrinking at boundaries.
func CandleWindow(bars []Bar, capacity, offset int) []Bar {
	if len(bars) == 0 {
		return nil
	}
	capacity = max(1, min(capacity, len(bars)))
	offset = max(0, min(offset, len(bars)-capacity))
	end := len(bars) - offset
	return bars[end-capacity : end]
}

// PointWindow returns a fixed-capacity slice ending offset steps before the latest point.
func PointWindow(points []Point, capacity, offset int) []Point {
	if len(points) == 0 {
		return nil
	}
	capacity = max(1, min(capacity, len(points)))
	offset = max(0, min(offset, len(points)-capacity))
	end := len(points) - offset
	return points[end-capacity : end]
}
