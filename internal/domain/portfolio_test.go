package domain

import (
	"testing"
	"time"

	"portifolio/internal/config"
)

func TestBuildPortfolioNeverAndMonthly(t *testing.T) {
	dates := []string{"2024-01-02", "2024-01-31", "2024-02-01", "2024-02-29"}
	a, b := AssetSeries{Position: config.Position{Allocation: .5}}, AssetSeries{Position: config.Position{Allocation: .5}}
	for i, d := range dates {
		dt, _ := time.Parse("2006-01-02", d)
		a.Points = append(a.Points, Point{dt, []float64{100, 110, 120, 130}[i]})
		b.Points = append(b.Points, Point{dt, 100})
	}
	p := config.Portfolio{StartDate: "2024-01-01", Rebalance: "never"}
	never := BuildPortfolio(p, []AssetSeries{a, b})
	if len(never) != 4 || never[0].Value != 100 || never[3].Value <= never[2].Value {
		t.Fatalf("unexpected buy-and-hold curve: %#v", never)
	}
	p.Rebalance = "monthly"
	monthly := BuildPortfolio(p, []AssetSeries{a, b})
	if len(monthly) != 4 || monthly[3].Value == never[3].Value {
		t.Fatalf("monthly rebalance not applied: never=%v monthly=%v", never[3], monthly[3])
	}
}
func TestBuildPortfolioStartsAtFirstCommonPrice(t *testing.T) {
	d1, _ := time.Parse("2006-01-02", "2024-01-02")
	d2, _ := time.Parse("2006-01-02", "2024-01-03")
	a := AssetSeries{Position: config.Position{Allocation: .5}, Points: []Point{{d1, 10}, {d2, 11}}}
	b := AssetSeries{Position: config.Position{Allocation: .5}, Points: []Point{{d2, 20}}}
	curve := BuildPortfolio(config.Portfolio{StartDate: "2024-01-01", Rebalance: "never"}, []AssetSeries{a, b})
	if len(curve) != 1 || !curve[0].Date.Equal(d2) || curve[0].Value != 100 {
		t.Fatalf("unexpected curve: %#v", curve)
	}
}
func TestAggregateUsesLastPointInBucket(t *testing.T) {
	var points []Point
	for _, d := range []string{"2024-01-02", "2024-01-31", "2024-02-01"} {
		dt, _ := time.Parse("2006-01-02", d)
		points = append(points, Point{dt, float64(dt.Day())})
	}
	got := Aggregate(points, "month")
	if len(got) != 2 || got[0].Value != 31 {
		t.Fatalf("expected month-end values, got %#v", got)
	}
}
