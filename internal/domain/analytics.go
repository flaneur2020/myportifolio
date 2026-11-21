package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"portifolio/internal/config"
)

type ContributionPeriod struct {
	Date   time.Time
	Assets []float64
	Total  float64
}

type AnalysisData struct {
	Portfolio          config.Portfolio
	ConfiguredWeight   float64
	Curve              []Point
	Snapshots          []PortfolioSnapshot
	Targets            []float64
	EffectiveStartNote string
	MaxDrawdown        float64
	Capital            float64
}

// BuildAnalysisData normalizes a copy of p, validates available price histories,
// and computes snapshots and curve in a single simulation pass.
func BuildAnalysisData(p config.Portfolio, bars map[string][]Bar) (AnalysisData, string) {
	if len(p.Positions) == 0 {
		return AnalysisData{}, "Press a to Add positions, then assign target weights in My Portfolio."
	}
	norm, total, ok := NormalizedPortfolio(p)
	if !ok {
		return AnalysisData{}, "Set at least one positive target weight before analysis."
	}
	var ss []AssetSeries
	var targets []float64
	for _, pos := range norm.Positions {
		if pos.Allocation == 0 {
			continue
		}
		assetBars := bars[pos.Symbol]
		if len(assetBars) < 2 {
			return AnalysisData{}, "History missing for " + pos.Symbol + ". Press r to retry."
		}
		pts := make([]Point, 0, len(assetBars))
		for _, bar := range assetBars {
			v := bar.AdjustedClose
			if v <= 0 {
				v = bar.Close
			}
			pts = append(pts, Point{Date: bar.Date, Value: v})
		}
		ss = append(ss, AssetSeries{Position: pos, Points: pts})
		targets = append(targets, pos.Allocation)
	}
	snapshots := BuildPortfolioSnapshots(norm, ss)
	curve := CurveFromSnapshots(snapshots)
	if len(curve) < 2 {
		return AnalysisData{}, "No common history for this portfolio."
	}
	peak, maxDD := curve[0].Value, 0.0
	for _, pt := range curve {
		peak = max(peak, pt.Value)
		maxDD = min(maxDD, pt.Value/peak-1)
	}
	capital := norm.InitialCapital
	if capital <= 0 {
		capital = 10000
	}
	return AnalysisData{
		Portfolio:          norm,
		ConfiguredWeight:   total,
		Curve:              curve,
		Snapshots:          snapshots,
		Targets:            targets,
		EffectiveStartNote: EffectiveStartNote(norm.StartDate, curve[0].Date, ss),
		MaxDrawdown:        maxDD,
		Capital:            capital,
	}, ""
}

func DrawdownSeries(curve []Point) []Point {
	var out []Point
	peak := 0.0
	for _, p := range curve {
		peak = max(peak, p.Value)
		if peak > 0 {
			out = append(out, Point{Date: p.Date, Value: (p.Value/peak - 1) * 100})
		}
	}
	return out
}

// AggregateDrawdown preserves the deepest daily drawdown in each interval.
func AggregateDrawdown(points []Point, grain Grain) []Point {
	var out []Point
	for _, p := range points {
		start := PeriodStart(p.Date, grain)
		if len(out) == 0 || !PeriodStart(out[len(out)-1].Date, grain).Equal(start) {
			out = append(out, p)
		} else {
			last := &out[len(out)-1]
			last.Value = min(last.Value, p.Value)
			last.Date = p.Date
		}
	}
	return out
}

// PeriodReturns compares successive period-end index values.
// The initial bucket uses the first model observation and may be a partial period.
func PeriodReturns(curve []Point, grain Grain) []Point {
	if len(curve) == 0 {
		return nil
	}
	ends := AggregateCandles(PortfolioCandles(curve), grain)
	previous := curve[0].Value
	var out []Point
	for _, end := range ends {
		if previous > 0 {
			out = append(out, Point{Date: end.Date, Value: (end.Close/previous - 1) * 100})
		}
		previous = end.Close
	}
	return out
}

// PeriodGains returns monetary changes in model value for each period.
func PeriodGains(curve []Point, grain Grain, capital float64) []Point {
	if len(curve) == 0 || curve[0].Value <= 0 {
		return nil
	}
	ends := AggregateCandles(PortfolioCandles(curve), grain)
	previous := curve[0].Value
	var out []Point
	for _, end := range ends {
		out = append(out, Point{Date: end.Date, Value: capital * (end.Close - previous) / curve[0].Value})
		previous = end.Close
	}
	return out
}

// RecentChange reports the latest model session's return and implied monetary change.
func RecentChange(curve []Point, initialCapital float64) (percent, amount float64, ok bool) {
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

func MaxDriftSeries(snapshots []PortfolioSnapshot, targets []float64, grain Grain) []Point {
	if len(snapshots) == 0 || len(targets) == 0 {
		return nil
	}
	type bucket struct {
		date  time.Time
		value float64
	}
	buckets := map[time.Time]*bucket{}
	for _, snapshot := range snapshots {
		if len(snapshot.Weights) != len(targets) {
			continue
		}
		drift := 0.0
		for i, weight := range snapshot.Weights {
			drift = max(drift, math.Abs(weight-targets[i]))
		}
		key := PeriodStart(snapshot.Date, grain)
		if existing, ok := buckets[key]; ok {
			existing.value = max(existing.value, drift)
			existing.date = snapshot.Date
		} else {
			buckets[key] = &bucket{date: snapshot.Date, value: drift}
		}
	}
	keys := make([]time.Time, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
	out := make([]Point, 0, len(keys))
	for _, key := range keys {
		b := buckets[key]
		out = append(out, Point{Date: b.date, Value: b.value * 100})
	}
	return out
}

// ContributionHistory attributes each day's model return using the weights in
// force for that day. On a rebalance day those are the target weights.
func ContributionHistory(p config.Portfolio, snapshots []PortfolioSnapshot, targets []float64, grain Grain) []ContributionPeriod {
	if len(snapshots) < 2 || len(targets) == 0 {
		return nil
	}
	type bucket struct {
		date   time.Time
		assets []float64
		total  float64
	}
	rbGrain, doRebalance := rebalanceGrain(p.Rebalance)
	buckets := map[time.Time]*bucket{}
	for i := 1; i < len(snapshots); i++ {
		previous, current := snapshots[i-1], snapshots[i]
		if len(previous.Weights) != len(targets) || len(previous.Prices) != len(targets) || len(current.Prices) != len(targets) {
			continue
		}
		rebalanced := doRebalance && PeriodKey(current.Date, rbGrain) != PeriodKey(previous.Date, rbGrain)
		weights := previous.Weights
		if rebalanced {
			weights = targets
		}
		key := PeriodStart(current.Date, grain)
		entry := buckets[key]
		if entry == nil {
			entry = &bucket{date: current.Date, assets: make([]float64, len(targets))}
			buckets[key] = entry
		}
		entry.date = current.Date
		for j := range targets {
			if previous.Prices[j] <= 0 || current.Prices[j] <= 0 {
				continue
			}
			contribution := weights[j] * (current.Prices[j]/previous.Prices[j] - 1) * 100
			entry.assets[j] += contribution
			entry.total += contribution
		}
	}
	keys := make([]time.Time, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
	out := make([]ContributionPeriod, 0, len(keys))
	for _, key := range keys {
		entry := buckets[key]
		out = append(out, ContributionPeriod{Date: entry.date, Assets: entry.assets, Total: entry.total})
	}
	return out
}

func EffectiveStartNote(requested string, effective time.Time, series []AssetSeries) string {
	start, err := time.Parse("2006-01-02", requested)
	if err != nil || !effective.After(start) {
		return ""
	}
	var constraints []string
	for _, asset := range series {
		var first time.Time
		for _, point := range asset.Points {
			if point.Value > 0 && !point.Date.Before(start) && (first.IsZero() || point.Date.Before(first)) {
				first = point.Date
			}
		}
		if !first.IsZero() && first.After(start.AddDate(0, 0, 7)) {
			constraints = append(constraints, fmt.Sprintf("%s history begins %s", asset.Position.Symbol, first.Format("2006-01-02")))
		}
	}
	if len(constraints) == 0 {
		return ""
	}
	return fmt.Sprintf("Effective start %s · %s. Earlier dates require prices for every weighted asset.", effective.Format("2006-01-02"), strings.Join(constraints, "; "))
}
