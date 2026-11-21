package ui

import (
	"portifolio/internal/config"
	"portifolio/internal/provider"
	"strings"
	"testing"
	"time"
)

func TestHistoryOnlyInsideSearchCards(t *testing.T) {
	c, _ := config.New("us")
	c.Portfolios[0].Positions = []config.Position{{Symbol: "VTI", Market: "us", Kind: "etf", Currency: "USD", Allocation: 1}}
	m := New(&c, "", nil, nil)
	m.width = 100
	m.height = 60
	m.assets = testAssets()
	m.bars["VTI"] = []provider.Bar{{Date: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), Open: 100, High: 105, Low: 95, Close: 104}, {Date: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC), Open: 104, High: 106, Low: 98, Close: 99}}
	for _, p := range []page{holdingsPage, addPage} {
		m.page = p
		view := m.View()
		if p == holdingsPage && strings.Contains(view, "█") {
			t.Fatal("portfolio summary should not contain candlesticks")
		}
		if p == addPage && !strings.Contains(view, "█") {
			t.Fatal("search card history missing")
		}
		if strings.Contains(view, "PRICE PREVIEW") {
			t.Fatal("detached preview remains")
		}
	}
}
func TestCardHistoryLoadingAndFailure(t *testing.T) {
	m := Model{priceLoading: map[string]bool{"SPY": true}, bars: map[string][]provider.Bar{}, errors: map[string]string{}}
	if !strings.Contains(m.cardHistory("SPY", 80), "Loading price history") {
		t.Fatal("loading state missing")
	}
	delete(m.priceLoading, "SPY")
	m.errors["SPY"] = "offline"
	if !strings.Contains(m.cardHistory("SPY", 80), "History unavailable") {
		t.Fatal("failure state missing")
	}
}
