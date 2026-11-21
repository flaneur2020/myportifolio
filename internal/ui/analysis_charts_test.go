package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"math"
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"portifolio/internal/provider"
	"strings"
	"testing"
	"time"
)

func TestSupplementalAnalysisCalculations(t *testing.T) {
	curve := []domain.Point{{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Value: 100}, {Date: time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), Value: 120}, {Date: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), Value: 90}, {Date: time.Date(2024, 3, 29, 0, 0, 0, 0, time.UTC), Value: 120}}
	dd := drawdownSeries(curve)
	if dd[2].Value != -25 || dd[3].Value != 0 {
		t.Fatalf("incorrect drawdown: %#v", dd)
	}
	returns := periodReturns(curve, "month")
	if len(returns) != 3 || math.Abs(returns[0].Value-20) > 1e-8 || returns[1].Value != -25 || math.Abs(returns[2].Value-100.0/3) > 1e-8 {
		t.Fatalf("incorrect monthly returns: %#v", returns)
	}
}
func TestPeriodCurrencyGainsAndInstantInterval(t *testing.T) {
	curve := []domain.Point{
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Value: 100},
		{Date: time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), Value: 120},
		{Date: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), Value: 90},
	}
	gains := periodGains(curve, "month", 10000)
	if len(gains) != 2 || gains[0].Value != 2000 || gains[1].Value != -3000 {
		t.Fatalf("incorrect monetary gains: %#v", gains)
	}
	// February loses 25% of 12,000, not 25% of the initial 10,000.
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m.page = analysisPage
	m.analysisCard = 4
	m.grain = "month"
	view := m.additionalAnalysisCharts(c.Portfolios[0], curve, 100)
	for _, want := range []string{"+2000.00 USD", "-3000.00 USD", "-25.00%"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q: %s", want, view)
		}
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if m.grain != "quarter" || m.analysisCard != 4 {
		t.Fatal("z did not switch selected period-return chart immediately")
	}
	quarter := periodGains(curve, m.grain, 10000)
	if len(quarter) != 1 || quarter[0].Value != -1000 {
		t.Fatalf("incorrect quarterly currency return: %#v", quarter)
	}
}

func TestPeriodReturnsDisplayedNewestFirst(t *testing.T) {
	curve := []domain.Point{
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Value: 100},
		{Date: time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), Value: 120},
		{Date: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), Value: 90},
		{Date: time.Date(2024, 3, 29, 0, 0, 0, 0, time.UTC), Value: 99},
	}
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m.grain = "month"
	for offset := 0; offset < 2; offset++ {
		m.chartOffsets[4] = offset
		view := m.additionalAnalysisCharts(c.Portfolios[0], curve, 100)
		view = view[strings.Index(view, "PERIOD RETURNS"):]
		march, feb, jan := strings.Index(view, "2024-03-29"), strings.Index(view, "2024-02-29"), strings.Index(view, "2024-01-31")
		if march < 0 || feb <= march || jan <= feb {
			t.Fatalf("returns not newest first: %s", view)
		}
		if !strings.Contains(view, "+900.00 USD") || !strings.Contains(view, "-3000.00 USD") {
			t.Fatal("currency gains lost their period association")
		}
	}
}

func TestAnalysisCardsAndScrolling(t *testing.T) {
	c, _ := config.New("us")
	c.Portfolios[0].StartDate = "2024-01-01"
	c.Portfolios[0].Positions = []config.Position{{Symbol: "VTI", Allocation: 1}}
	m := New(&c, "", nil, nil)
	m.page = analysisPage
	m.height = 24
	m.width = 100
	m.bars["VTI"] = []provider.Bar{{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Close: 100}, {Date: time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC), Close: 110}}
	full := m.analysisView(100)
	for _, heading := range []string{"PORTFOLIO INFORMATION", "PORTFOLIO HISTORY", "DRAWDOWN / HISTORY", "TARGET ALLOCATION", "PERIOD RETURNS"} {
		if !strings.Contains(full, heading) {
			t.Fatalf("missing chart %q", heading)
		}
	}
	before := m.View()
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyPgDown})
	if m.analysisScroll == 0 || m.View() == before {
		t.Fatal("page cannot scroll")
	}
	if m.grain != "day" || m.offset != 0 {
		t.Fatal("scroll changed chart controls")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyPgUp})
	if m.analysisScroll != 0 {
		t.Fatal("cannot return to top")
	}
}
