package core

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestBuildPortfolioNeverAndMonthly(t *testing.T) {
	dates := []string{"2024-01-02", "2024-01-31", "2024-02-01", "2024-02-29"}
	a, b := AssetSeries{Position: Position{Allocation: .5}}, AssetSeries{Position: Position{Allocation: .5}}
	for i, d := range dates {
		dt, _ := time.Parse("2006-01-02", d)
		a.Points = append(a.Points, Point{dt, []float64{100, 110, 120, 130}[i]})
		b.Points = append(b.Points, Point{dt, 100})
	}
	p := Portfolio{StartDate: "2024-01-01", Rebalance: RebalanceNever}
	never := BuildPortfolio(p, []AssetSeries{a, b})
	if len(never) != 4 || never[0].Value != 100 || never[3].Value <= never[2].Value {
		t.Fatalf("unexpected buy-and-hold curve: %#v", never)
	}
	p.Rebalance = RebalanceMonthly
	monthly := BuildPortfolio(p, []AssetSeries{a, b})
	if len(monthly) != 4 || monthly[3].Value == never[3].Value {
		t.Fatalf("monthly rebalance not applied: never=%v monthly=%v", never[3], monthly[3])
	}
}

func TestBuildPortfolioStartsAtFirstCommonPrice(t *testing.T) {
	d1, _ := time.Parse("2006-01-02", "2024-01-02")
	d2, _ := time.Parse("2006-01-02", "2024-01-03")
	a := AssetSeries{Position: Position{Allocation: .5}, Points: []Point{{d1, 10}, {d2, 11}}}
	b := AssetSeries{Position: Position{Allocation: .5}, Points: []Point{{d2, 20}}}
	curve := BuildPortfolio(Portfolio{StartDate: "2024-01-01", Rebalance: RebalanceNever}, []AssetSeries{a, b})
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
	got := Aggregate(points, GrainMonth)
	if len(got) != 2 || got[0].Value != 31 {
		t.Fatalf("expected month-end values, got %#v", got)
	}
}

func TestSnapshotsExposeWeightsAndRebalance(t *testing.T) {
	a := AssetSeries{Position: Position{Allocation: .5}, Points: []Point{{day(2024, 1, 2), 100}, {day(2024, 2, 1), 100}, {day(2024, 2, 2), 100}, {day(2024, 3, 1), 100}}}
	b := AssetSeries{Position: Position{Allocation: .5}, Points: []Point{{day(2024, 1, 2), 100}, {day(2024, 2, 1), 200}, {day(2024, 2, 2), 200}, {day(2024, 3, 1), 200}}}
	never := BuildPortfolioSnapshots(Portfolio{StartDate: "2024-01-01", Rebalance: RebalanceNever}, []AssetSeries{a, b})
	if len(never) != 4 {
		t.Fatal("missing snapshots")
	}
	if math.Abs(never[2].Weights[0]-1.0/3) > 1e-9 || math.Abs(never[2].Weights[1]-2.0/3) > 1e-9 {
		t.Fatalf("wrong drift weights: %#v", never[2].Weights)
	}
	monthly := BuildPortfolioSnapshots(Portfolio{StartDate: "2024-01-01", Rebalance: RebalanceMonthly}, []AssetSeries{a, b})
	last := monthly[len(monthly)-1]
	if math.Abs(last.Weights[0]-.5) > 1e-9 || math.Abs(last.Weights[1]-.5) > 1e-9 {
		t.Fatalf("rebalance did not restore weights: %#v", last.Weights)
	}
}

func TestDrawdownReturnsGainsAndRecentChange(t *testing.T) {
	curve := []Point{
		{Date: day(2024, 1, 2), Value: 100},
		{Date: day(2024, 1, 31), Value: 120},
		{Date: day(2024, 2, 29), Value: 90},
		{Date: day(2024, 3, 29), Value: 120},
	}
	dd := DrawdownSeries(curve)
	if dd[2].Value != -25 || dd[3].Value != 0 {
		t.Fatalf("incorrect drawdown: %#v", dd)
	}
	returns := PeriodReturns(curve, GrainMonth)
	if len(returns) != 3 || math.Abs(returns[0].Value-20) > 1e-8 || returns[1].Value != -25 || math.Abs(returns[2].Value-100.0/3) > 1e-8 {
		t.Fatalf("incorrect monthly returns: %#v", returns)
	}
	gains := PeriodGains(curve[:3], GrainMonth, 10000)
	if len(gains) != 2 || gains[0].Value != 2000 || gains[1].Value != -3000 {
		t.Fatalf("incorrect monetary gains: %#v", gains)
	}
	quarter := PeriodGains(curve[:3], GrainQuarter, 10000)
	if len(quarter) != 1 || quarter[0].Value != -1000 {
		t.Fatalf("incorrect quarterly currency return: %#v", quarter)
	}
	agg := AggregateDrawdown([]Point{
		{Date: day(2024, 1, 2), Value: 0},
		{Date: day(2024, 1, 15), Value: -20},
		{Date: day(2024, 1, 31), Value: -5},
		{Date: day(2024, 2, 1), Value: -3},
	}, GrainMonth)
	if len(agg) != 2 || agg[0].Value != -20 || agg[0].Date.Day() != 31 {
		t.Fatalf("grouping lost intraperiod trough: %#v", agg)
	}

	pct, amt, ok := RecentChange([]Point{
		{Date: day(2024, 1, 2), Value: 100},
		{Date: day(2024, 1, 3), Value: 110},
		{Date: day(2024, 1, 4), Value: 99},
	}, 10000)
	if !ok || math.Abs(pct+10) > 1e-9 || amt != -1100 {
		t.Fatalf("incorrect recent change: %v %v", pct, amt)
	}
}

func TestContributionCandlesAndEffectiveStart(t *testing.T) {
	p := Portfolio{
		StartDate: "2024-01-01",
		Rebalance: RebalanceNever,
		Positions: []Position{{Symbol: "AAA", Allocation: .5}, {Symbol: "BBB", Allocation: .5}},
	}
	start := day(2024, 1, 2)
	series := []AssetSeries{
		{Position: p.Positions[0], Points: []Point{{start, 100}, {start.AddDate(0, 0, 1), 110}, {start.AddDate(0, 0, 2), 110}}},
		{Position: p.Positions[1], Points: []Point{{start, 100}, {start.AddDate(0, 0, 1), 90}, {start.AddDate(0, 0, 2), 90}}},
	}
	snaps := BuildPortfolioSnapshots(p, series)
	periods := ContributionHistory(p, snaps, []float64{.5, .5}, GrainDay)
	if len(periods) != 2 || math.Abs(periods[0].Assets[0]-5) > 1e-9 || math.Abs(periods[0].Total) > 1e-9 {
		t.Fatalf("wrong contributions: %#v", periods)
	}

	bars := []Bar{
		{Date: day(2025, 1, 6), Open: 100, High: 110, Low: 95, Close: 105},
		{Date: day(2025, 1, 7), Open: 105, High: 115, Low: 99, Close: 101},
		{Date: day(2025, 1, 13), Open: 101, High: 103, Low: 90, Close: 92},
	}
	got := AggregateCandles(bars, GrainWeek)
	if len(got) != 2 || got[0].Open != 100 || got[0].High != 115 || got[0].Low != 95 || got[0].Close != 101 {
		t.Fatalf("incorrect weekly candle: %#v", got)
	}
	early, late := day(2020, 1, 2), day(2020, 6, 1)
	series2 := []AssetSeries{
		{Position: Position{Symbol: "SPY", Allocation: .5}, Points: []Point{{early, 100}, {late, 110}}},
		{Position: Position{Symbol: "SGOV", Allocation: .5}, Points: []Point{{late, 100}}},
	}
	curve := BuildPortfolio(Portfolio{StartDate: "2020-01-01", Rebalance: RebalanceNever}, series2)
	note := EffectiveStartNote("2020-01-01", curve[0].Date, series2)
	if !strings.Contains(note, "SGOV history begins 2020-06-01") {
		t.Fatalf("missing explanation: %s", note)
	}
	end := day(2026, 6, 15)
	prev := end
	for _, g := range Grains {
		s := RangeStart(end, g)
		if !s.Before(prev) {
			t.Fatalf("%s did not expand history", g)
		}
		prev = s
	}
}

type stubPriceProvider struct {
	name   string
	fail   bool
	called *int
}

func (s stubPriceProvider) Name() string      { return s.name }
func (s stubPriceProvider) Markets() []string { return []string{"crypto"} }
func (s stubPriceProvider) History(context.Context, Position, time.Time, time.Time) ([]Bar, error) {
	*s.called++
	if s.fail {
		return nil, errors.New("provider failed")
	}
	return []Bar{{Date: day(2024, 1, 2), Close: 100, Source: s.name}}, nil
}

func TestKernelDynamicMarketRegistryFallbackAndOverride(t *testing.T) {
	k := NewKernel()
	if err := k.RegisterMarket(MarketSpec{
		ID:             "crypto",
		Name:           "Crypto",
		Currency:       "USDT",
		AssetKinds:     []string{"spot"},
		PriceProviders: []string{"primary", "backup"},
	}, false); err != nil {
		t.Fatal(err)
	}
	// Duplicate without override is rejected.
	if err := k.RegisterMarket(MarketSpec{ID: "crypto", Currency: "USDT"}, false); err == nil {
		t.Fatal("duplicate market accepted without override")
	}
	// Override succeeds.
	if err := k.RegisterMarket(MarketSpec{
		ID:             "crypto",
		Name:           "Crypto Spot",
		Currency:       "USDT",
		AssetKinds:     []string{"spot"},
		PriceProviders: []string{"primary", "backup"},
	}, true); err != nil {
		t.Fatal(err)
	}

	c, err := k.NewConfig("crypto")
	if err != nil {
		t.Fatal(err)
	}
	c.Portfolio.Positions = []Position{{Symbol: "BTC", Market: "crypto", Kind: "spot", Currency: "USDT", Allocation: 0.6}}
	if err := k.ValidateConfig(c); err != nil {
		t.Fatalf("valid custom market config rejected: %v", err)
	}
	c.Portfolio.Positions[0].Kind = "unsupported"
	if err := k.ValidateConfig(c); err == nil {
		t.Fatal("disallowed asset kind accepted")
	}

	p1, p2 := 0, 0
	_ = k.RegisterPriceProvider(stubPriceProvider{"primary", true, &p1}, false)
	_ = k.RegisterPriceProvider(stubPriceProvider{"backup", false, &p2}, false)
	bars, err := k.FetchHistory(context.Background(), Position{Market: "crypto", Symbol: "BTC"}, day(2024, 1, 1), day(2024, 1, 3))
	if err != nil || len(bars) != 1 || p1 != 1 || p2 != 1 {
		t.Fatalf("fallback failed: bars=%v err=%v p1=%d p2=%d", bars, err, p1, p2)
	}
}
