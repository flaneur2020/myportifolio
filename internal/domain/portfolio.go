package domain

import (
	"sort"
	"time"

	"portifolio/internal/config"
)

type Point struct {
	Date  time.Time
	Value float64
}
type AssetSeries struct {
	Position config.Position
	Points   []Point
}

func BuildPortfolio(p config.Portfolio, series []AssetSeries) []Point {
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
	out := make([]Point, 0, len(dates)-first)
	lastPeriod := ""
	for _, d := range dates[first:] {
		date, _ := time.Parse("2006-01-02", d)
		period := rebalancePeriod(date, p.Rebalance)
		if len(out) > 0 && p.Rebalance != "never" && period != lastPeriod {
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
			out = append(out, Point{Date: date, Value: value})
		}
		lastPeriod = period
	}
	if len(out) == 0 {
		return nil
	}
	base := out[0].Value
	for i := range out {
		out[i].Value = out[i].Value / base * 100
	}
	return out
}

func rebalancePeriod(t time.Time, rule string) string {
	switch rule {
	case "monthly":
		return t.Format("2006-01")
	case "quarterly":
		return t.Format("2006") + "-Q" + string(rune('1'+(int(t.Month())-1)/3))
	case "yearly":
		return t.Format("2006")
	}
	return ""
}
func Aggregate(points []Point, grain string) []Point {
	if grain == "day" {
		return points
	}
	buckets := map[string]Point{}
	for _, p := range points {
		key := p.Date.Format("2006-01-02")
		switch grain {
		case "week":
			y, w := p.Date.ISOWeek()
			key = time.Date(y, 1, 4, 0, 0, 0, 0, time.UTC).AddDate(0, 0, (w-1)*7).Format("2006-01-02")
		case "month":
			key = p.Date.Format("2006-01")
		case "quarter":
			key = p.Date.Format("2006") + "-Q" + string(rune('1'+(int(p.Date.Month())-1)/3))
		}
		buckets[key] = p
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
