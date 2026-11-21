package domain

import (
	"math"
	"strings"
	"testing"
	"time"

	"portifolio/internal/config"
)

func TestDrawdownReturnsAndGains(t *testing.T) {
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
}

func TestAggregateDrawdownKeepsWorstDailyDecline(t *testing.T) {
	points := []Point{
		{Date: day(2024, 1, 2), Value: 0},
		{Date: day(2024, 1, 15), Value: -20},
		{Date: day(2024, 1, 31), Value: -5},
		{Date: day(2024, 2, 1), Value: -3},
	}
	got := AggregateDrawdown(points, GrainMonth)
	if len(got) != 2 || got[0].Value != -20 || got[0].Date.Day() != 31 {
		t.Fatalf("grouping lost intraperiod trough: %#v", got)
	}
}

func TestRecentChange(t *testing.T) {
	curve := []Point{
		{Date: day(2024, 1, 2), Value: 100},
		{Date: day(2024, 1, 3), Value: 110},
		{Date: day(2024, 1, 4), Value: 99},
	}
	percent, amount, ok := RecentChange(curve, 10000)
	if !ok || math.Abs(percent+10) > 1e-9 || amount != -1100 {
		t.Fatalf("incorrect recent change: %v %v", percent, amount)
	}
	if _, _, ok := RecentChange(curve[:1], 10000); ok {
		t.Fatal("single point produced a daily change")
	}
	if _, _, ok := RecentChange(curve, 0); ok {
		t.Fatal("missing capital produced a currency change")
	}
}

func TestContributionAttributesDailyReturns(t *testing.T) {
	p := config.Portfolio{
		StartDate: "2024-01-01",
		Rebalance: "never",
		Positions: []config.Position{{Symbol: "AAA", Allocation: .5}, {Symbol: "BBB", Allocation: .5}},
	}
	start := day(2024, 1, 2)
	series := []AssetSeries{
		{Position: p.Positions[0], Points: []Point{{start, 100}, {start.AddDate(0, 0, 1), 110}, {start.AddDate(0, 0, 2), 110}}},
		{Position: p.Positions[1], Points: []Point{{start, 100}, {start.AddDate(0, 0, 1), 90}, {start.AddDate(0, 0, 2), 90}}},
	}
	snaps := BuildPortfolioSnapshots(p, series)
	periods := ContributionHistory(p, snaps, []float64{.5, .5}, GrainDay)
	if len(periods) != 2 {
		t.Fatalf("expected two return days: %#v", periods)
	}
	if math.Abs(periods[0].Assets[0]-5) > 1e-9 || math.Abs(periods[0].Assets[1]+5) > 1e-9 || math.Abs(periods[0].Total) > 1e-9 {
		t.Fatalf("wrong contributions: %#v", periods[0])
	}
	if math.Abs(periods[1].Total) > 1e-9 {
		t.Fatalf("flat day should contribute zero: %#v", periods[1])
	}
}

func TestCandlesAndWindows(t *testing.T) {
	bars := []Bar{
		{Date: day(2025, 1, 6), Open: 100, High: 110, Low: 95, Close: 105},
		{Date: day(2025, 1, 7), Open: 105, High: 115, Low: 99, Close: 101},
		{Date: day(2025, 1, 13), Open: 101, High: 103, Low: 90, Close: 92},
	}
	got := AggregateCandles(bars, GrainWeek)
	if len(got) != 2 || got[0].Open != 100 || got[0].High != 115 || got[0].Low != 95 || got[0].Close != 101 {
		t.Fatalf("incorrect weekly candle: %#v", got)
	}

	yearly := []Bar{
		{Date: day(2024, 1, 2), Open: 100, High: 120, Low: 90, Close: 110},
		{Date: day(2024, 12, 30), Open: 110, High: 150, Low: 105, Close: 140},
		{Date: day(2025, 1, 2), Open: 140, High: 145, Low: 130, Close: 135},
	}
	gotYear := AggregateCandles(yearly, GrainYear)
	if len(gotYear) != 2 || gotYear[0].Open != 100 || gotYear[0].Close != 140 || gotYear[0].High != 150 || gotYear[0].Low != 90 {
		t.Fatalf("incorrect yearly OHLC: %#v", gotYear)
	}

	var many []Bar
	for i := 0; i < 100; i++ {
		many = append(many, Bar{Date: time.Date(2025, 1, 1+i, 0, 0, 0, 0, time.UTC), Close: 100 + float64(i)})
	}
	latest := CandleWindow(many, 20, 0)
	prev := CandleWindow(many, 20, 1)
	if len(latest) != 20 || len(prev) != 20 || !prev[1].Date.Equal(latest[0].Date) || !prev[19].Date.Equal(latest[18].Date) {
		t.Fatal("window must move one candle without resizing")
	}
	earliest := CandleWindow(many, 20, 1000)
	if len(earliest) != 20 || !earliest[0].Date.Equal(many[0].Date) {
		t.Fatal("earliest boundary shrank the window")
	}
}

func TestEffectiveStartNoteAndRangeStart(t *testing.T) {
	early := day(2020, 1, 2)
	late := day(2020, 6, 1)
	series := []AssetSeries{
		{Position: config.Position{Symbol: "SPY", Allocation: .5}, Points: []Point{{early, 100}, {late, 110}}},
		{Position: config.Position{Symbol: "SGOV", Allocation: .5}, Points: []Point{{late, 100}}},
	}
	curve := BuildPortfolio(config.Portfolio{StartDate: "2020-01-01", Rebalance: "never"}, series)
	note := EffectiveStartNote("2020-01-01", curve[0].Date, series)
	if !strings.Contains(note, "SGOV history begins 2020-06-01") || !strings.Contains(note, "Effective start 2020-06-01") {
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
