package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestDrawdownGroupingKeepsWorstDailyDecline(t *testing.T) {
	points := []domain.Point{{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Value: 0}, {Date: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), Value: -20}, {Date: time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), Value: -5}, {Date: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), Value: -3}}
	got := aggregateDrawdown(points, "month")
	if len(got) != 2 || got[0].Value != -20 || got[0].Date.Day() != 31 {
		t.Fatalf("grouping lost intraperiod trough: %#v", got)
	}
	view := renderDrawdown(got, 40, 7)
	for _, want := range []string{"0.00%", "-20.00%", "└", "2024-01-31", "░", "━"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q: %s", want, view)
		}
	}
}
func TestDrawdownZUpdatesInterval(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m.page = analysisPage
	m.analysisCard = 2
	curve := []domain.Point{{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Value: 100}, {Date: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), Value: 90}}
	for _, want := range []string{"week", "month"} {
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
		if m.grain != want || !strings.Contains(m.drawdownCard(curve, 100), strings.ToUpper(want)) {
			t.Fatal("z did not update drawdown interval")
		}
	}
}
func TestFlatDrawdownAxis(t *testing.T) {
	view := renderDrawdown([]domain.Point{{Date: time.Now(), Value: 0}}, 20, 7)
	if strings.Contains(view, "NaN") || strings.Contains(view, "Inf") {
		t.Fatal("flat history produced invalid axis")
	}
}
