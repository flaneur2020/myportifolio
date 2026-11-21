package ui

import (
	"portifolio/internal/provider"
	"strings"
	"testing"
	"time"
)

func TestCandleAggregation(t *testing.T) {
	bars := []provider.Bar{
		{Date: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC), Open: 100, High: 110, Low: 95, Close: 105},
		{Date: time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC), Open: 105, High: 115, Low: 99, Close: 101},
		{Date: time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC), Open: 101, High: 103, Low: 90, Close: 92},
	}
	got := aggregateCandles(bars, "week")
	if len(got) != 2 || got[0].Open != 100 || got[0].High != 115 || got[0].Low != 95 || got[0].Close != 101 {
		t.Fatalf("incorrect weekly candle: %#v", got)
	}
	view := renderCandles(bars, "day", 60, 10)
	for _, label := range []string{"│", "█"} {
		if !strings.Contains(view, label) {
			t.Fatalf("chart missing %q", label)
		}
	}
	if strings.Contains(view, "latest") || strings.Contains(view, "Up / unchanged") {
		t.Fatal("chart footers still present")
	}
	if strings.Contains(view, "•") {
		t.Fatal("chart still uses dots")
	}
}
func TestPriceAnimationLifecycle(t *testing.T) {
	m := Model{priceLoading: map[string]bool{"SPY": true}, bars: map[string][]provider.Bar{}, errors: map[string]string{}}
	before := m.priceAnimation()
	next, cmd := m.Update(priceTick{})
	m = next.(Model)
	if cmd == nil || m.priceAnimation() == before {
		t.Fatal("price animation did not advance")
	}
	next, _ = m.Update(assetLoaded{symbol: "SPY"})
	m = next.(Model)
	if len(m.priceLoading) != 0 {
		t.Fatal("load did not finish")
	}
	_, cmd = m.Update(priceTick{})
	if cmd != nil {
		t.Fatal("animation continues after completion")
	}
}
