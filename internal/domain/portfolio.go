package domain

import (
	"sort"
	"time"

	"portifolio/internal/config"
)

type AssetSeries struct {
	Position config.Position
	Points   []Point
}

type PortfolioSnapshot struct {
	Date    time.Time
	Value   float64
	Prices  []float64
	Weights []float64
}

func TotalAllocation(positions []config.Position) float64 {
	total := 0.0
	for _, p := range positions {
		total += p.Allocation
	}
	return total
}

func PositivePositions(p config.Portfolio) []config.Position {
	var out []config.Position
	for _, pos := range p.Positions {
		if pos.Allocation > 0 {
			out = append(out, pos)
		}
	}
	return out
}

// NormalizedPortfolio returns a copy of p whose positive weights sum to 1.0,
// along with the original total allocation.
func NormalizedPortfolio(p config.Portfolio) (config.Portfolio, float64, bool) {
	total := TotalAllocation(p.Positions)
	if total <= 0 {
		return p, total, false
	}
	out := p
	out.Positions = append([]config.Position(nil), p.Positions...)
	for i := range out.Positions {
		out.Positions[i].Allocation /= total
	}
	return out, total, true
}

func CurveFromSnapshots(snapshots []PortfolioSnapshot) []Point {
	if len(snapshots) == 0 {
		return nil
	}
	base := snapshots[0].Value
	out := make([]Point, 0, len(snapshots))
	for _, snapshot := range snapshots {
		out = append(out, Point{Date: snapshot.Date, Value: snapshot.Value / base * 100})
	}
	return out
}

func BuildPortfolio(p config.Portfolio, series []AssetSeries) []Point {
	return CurveFromSnapshots(BuildPortfolioSnapshots(p, series))
}

// BuildPortfolioSnapshots exposes the daily prices, end-of-day weights and raw
// value used by the model. This lets analysis such as drift and contribution
// reuse exactly the same simulation as the portfolio curve.
func BuildPortfolioSnapshots(p config.Portfolio, series []AssetSeries) []PortfolioSnapshot {
	if len(series) == 0 {
		return nil
	}
	start, err := time.Parse("2006-01-02", p.StartDate)
	if err != nil {
		return nil
	}
	prices := make([]map[string]float64, len(series))
	dateSet := map[string]bool{}
	for i, s := range series {
		prices[i] = make(map[string]float64, len(s.Points))
		for _, pt := range s.Points {
			if !pt.Date.Before(start) && pt.Value > 0 {
				key := pt.Date.Format("2006-01-02")
				prices[i][key] = pt.Value
				dateSet[key] = true
			}
		}
	}
	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	first := -1
	for i, d := range dates {
		complete := true
		for _, asset := range prices {
			if asset[d] <= 0 {
				complete = false
				break
			}
		}
		if complete {
			first = i
			break
		}
	}
	if first < 0 {
		return nil
	}
	units := make([]float64, len(series))
	previous := make([]float64, len(series))
	for i, s := range series {
		previous[i] = prices[i][dates[first]]
		units[i] = s.Position.Allocation / previous[i]
	}
	rbGrain, doRebalance := rebalanceGrain(p.Rebalance)
	out := make([]PortfolioSnapshot, 0, len(dates)-first)
	lastPeriod := ""
	for _, d := range dates[first:] {
		date, _ := time.Parse("2006-01-02", d)
		period := ""
		if doRebalance {
			period = PeriodKey(date, rbGrain)
		}
		if len(out) > 0 && doRebalance && period != lastPeriod {
			value := 0.0
			for i := range units {
				value += units[i] * previous[i]
			}
			if value > 0 {
				for i, s := range series {
					units[i] = value * s.Position.Allocation / previous[i]
				}
			}
		}
		value := 0.0
		for i := range series {
			if current := prices[i][d]; current > 0 {
				previous[i] = current
			}
			value += units[i] * previous[i]
		}
		if value > 0 {
			pricesCopy := append([]float64(nil), previous...)
			weights := make([]float64, len(series))
			for i := range series {
				weights[i] = units[i] * previous[i] / value
			}
			out = append(out, PortfolioSnapshot{Date: date, Value: value, Prices: pricesCopy, Weights: weights})
		}
		lastPeriod = period
	}
	return out
}

func Aggregate(points []Point, grain Grain) []Point {
	if grain == GrainDay {
		return points
	}
	buckets := map[string]Point{}
	for _, p := range points {
		buckets[PeriodKey(p.Date, grain)] = p
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Point, 0, len(keys))
	for _, k := range keys {
		out = append(out, buckets[k])
	}
	return out
}
