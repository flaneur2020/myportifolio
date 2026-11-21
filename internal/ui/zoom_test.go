package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"portifolio/internal/provider"
	"testing"
	"time"
)

func TestAnalysisCardActivationAndControls(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m.page = analysisPage
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.analysisCard != 1 || m.analysisActive || m.grain != "day" {
		t.Fatal("Down must select card while inactive")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.offset != 0 {
		t.Fatal("inactive card must not pan")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.analysisActive || m.analysisCard != 1 {
		t.Fatal("Enter did not activate selected card")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.offset != 1 {
		t.Fatal("active history card did not pan")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.grain != "week" || m.analysisCard != 1 {
		t.Fatal("active Up/Down must change interval, not selection")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.grain != "day" {
		t.Fatal("active Up did not select finer interval")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.analysisActive || m.page != analysisPage {
		t.Fatal("Esc must return to card selection without leaving Analysis")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.analysisCard != 2 {
		t.Fatal("card navigation did not resume after Esc")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.chartOffsets[2] != 1 || m.offset != 1 {
		t.Fatal("active drawdown card panned the wrong chart")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if m.grain != "week" || m.analysisCard != 2 {
		t.Fatal("z must preserve active card")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.grain != "week" || m.analysisCard != 3 {
		t.Fatal("informational card must stay selected without changing interval")
	}
}
func TestCtrlArrowIntervalsPreserveSelectedCard(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m.page = analysisPage
	m.analysisCard = 2
	m.offset = 3
	for _, want := range []string{"week", "month", "quarter", "year", "year"} {
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlDown})
		if m.grain != want || m.analysisCard != 2 || m.offset != 3 {
			t.Fatalf("Ctrl+Down: grain=%s card=%d offset=%d", m.grain, m.analysisCard, m.offset)
		}
	}
	for _, want := range []string{"quarter", "month", "week", "day", "day"} {
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlUp})
		if m.grain != want {
			t.Fatalf("expected %s, got %s", want, m.grain)
		}
	}
	m.page = detailPage
	m.detailGrain = "month"
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlDown})
	if m.detailGrain != "quarter" {
		t.Fatal("asset details Ctrl+Down failed")
	}
}

func TestYearlyOHLC(t *testing.T) {
	bars := []provider.Bar{{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Open: 100, High: 120, Low: 90, Close: 110}, {Date: time.Date(2024, 12, 30, 0, 0, 0, 0, time.UTC), Open: 110, High: 150, Low: 105, Close: 140}, {Date: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), Open: 140, High: 145, Low: 130, Close: 135}}
	got := aggregateCandles(bars, "year")
	if len(got) != 2 || got[0].Open != 100 || got[0].Close != 140 || got[0].High != 150 || got[0].Low != 90 {
		t.Fatalf("incorrect yearly OHLC: %#v", got)
	}
}
