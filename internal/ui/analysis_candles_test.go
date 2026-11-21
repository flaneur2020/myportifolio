package ui

import (
	"portifolio/internal/domain"
	"portifolio/internal/provider"
	"strings"
	"testing"
	"time"
)

func TestCandleWindowMovesWithoutResizing(t *testing.T) {
	var bars []provider.Bar
	for i := 0; i < 100; i++ {
		bars = append(bars, provider.Bar{Date: time.Date(2025, 1, 1+i, 0, 0, 0, 0, time.UTC), Close: 100 + float64(i)})
	}
	latest := candleWindow(bars, 20, 0)
	previous := candleWindow(bars, 20, 1)
	if len(latest) != 20 || len(previous) != 20 || !previous[1].Date.Equal(latest[0].Date) || !previous[19].Date.Equal(latest[18].Date) {
		t.Fatal("window must move one candle without resizing")
	}
	earliest := candleWindow(bars, 20, 1000)
	if len(earliest) != 20 || !earliest[0].Date.Equal(bars[0].Date) {
		t.Fatal("earliest boundary shrank the window")
	}
}
func TestAnalysisOffsetMovesOneCandle(t *testing.T) {
	var curve []domain.Point
	for i := 0; i < 100; i++ {
		curve = append(curve, domain.Point{Date: time.Date(2025, 1, 1+i, 0, 0, 0, 0, time.UTC), Value: 100 + float64(i)})
	}
	m := Model{grain: "day", offset: 1}
	view := m.analysisCandles(curve, 80)
	want := curve[98].Date.Format("2006-01-02")
	if !strings.Contains(view, "→ "+want) {
		t.Fatalf("one step should end at %s: %s", want, view)
	}
}
func TestYearlyWindowLabelShowsCoverageNotClosingDate(t *testing.T) {
	curve := []domain.Point{
		{Date: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), Value: 100},
		{Date: time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC), Value: 120},
		{Date: time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC), Value: 125},
		{Date: time.Date(2021, 12, 31, 0, 0, 0, 0, time.UTC), Value: 130},
	}
	m := Model{grain: "year"}
	view := m.analysisCandles(curve, 80)
	if !strings.Contains(view, "Window: 2020-01-02 → 2021-12-31") {
		t.Fatalf("incorrect yearly coverage: %s", view)
	}
	// Four candles fit the minimum chart width; use enough years to crop 2020.
	for year := 2022; year <= 2024; year++ {
		curve = append(curve, domain.Point{Date: time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC), Value: 135})
	}
	view = m.analysisCandles(curve, 20)
	// On a narrow terminal the card wraps its text and adds border glyphs.
	compact := strings.NewReplacer(" ", "", "\n", "", "│", "").Replace(view)
	if !strings.Contains(compact, "Window:2021-01-04→2024-01-04") {
		t.Fatalf("incorrect panned coverage: %s", view)
	}
}

func TestPortfolioCandlesUseModelCloses(t *testing.T) {
	curve := []domain.Point{{Date: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Value: 100}, {Date: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), Value: 110}}
	candles := portfolioCandles(curve)
	if candles[1].Open != 100 || candles[1].Close != 110 || candles[1].High != 110 || candles[1].Low != 100 {
		t.Fatal("wrong model candle")
	}
	m := Model{grain: "day"}
	view := m.analysisCandles(curve, 80)
	if strings.Contains(view, "•") || !strings.Contains(view, "█") {
		t.Fatal("analysis does not use candlesticks")
	}
}
