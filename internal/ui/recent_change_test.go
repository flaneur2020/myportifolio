package ui

import (
	"math"
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"portifolio/internal/provider"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestRecentChangePercentageAndCurrency(t *testing.T) {
	curve := []domain.Point{{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Value: 100}, {Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Value: 110}, {Date: time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC), Value: 99}}
	percent, amount, ok := recentChange(curve, 10000)
	if !ok || math.Abs(percent+10) > 1e-9 || amount != -1100 {
		t.Fatalf("incorrect recent change: %v %v", percent, amount)
	}
	if _, _, ok := recentChange(curve[:1], 10000); ok {
		t.Fatal("single point produced a daily change")
	}
	if _, _, ok := recentChange(curve, 0); ok {
		t.Fatal("missing capital produced a currency change")
	}
}
func TestPortfolioInformationShowsRecentDayChange(t *testing.T) {
	c, _ := config.New("us")
	c.Portfolios[0].StartDate = "2024-01-01"
	c.Portfolios[0].InitialCapital = 10000
	c.Portfolios[0].Positions = []config.Position{{Symbol: "VTI", Allocation: 1}}
	m := New(&c, "", nil, nil)
	m.page = analysisPage
	m.bars["VTI"] = []provider.Bar{
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Close: 100},
		{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Close: 110},
		{Date: time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC), Close: 99},
	}
	view := m.analysisView(120)
	for _, want := range []string{"Recent day", "2024-01-04", "-10.00%", "-1100.00 USD"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
}

func TestPortfolioInformationColumnsAlign(t *testing.T) {
	c, _ := config.New("us")
	c.Portfolios[0].InitialCapital = 10000
	c.Portfolios[0].StartDate = "2024-01-01"
	c.Portfolios[0].Positions = []config.Position{{Symbol: "VTI", Allocation: 1}}
	m := New(&c, "", nil, nil)
	m.page = analysisPage
	m.bars["VTI"] = []provider.Bar{{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Close: 100}, {Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Close: 105}}
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.analysisView(120), "")
	index := strings.Index(plain, "PORTFOLIO INFORMATION")
	text := plain[index:]
	labels := []string{"Model index", "Total return", "Max drawdown", "Recent day", "Requested start", "Effective start", "Latest data", "Rebalance", "Initial capital", "Model value"}
	column := -1
	for _, label := range labels {
		line := label + strings.Repeat(" ", 17-len(label)) + "  "
		pos := strings.Index(text, line)
		if pos < 0 {
			t.Fatalf("missing aligned row %q", label)
		}
		lineStart := strings.LastIndex(text[:pos], "\n") + 1
		valueColumn := pos - lineStart + len(line)
		if column < 0 {
			column = valueColumn
		} else if valueColumn != column {
			t.Fatalf("%s value at %d, expected %d", label, valueColumn, column)
		}
	}
}
