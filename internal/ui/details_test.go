package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"portifolio/internal/provider"
	"strings"
	"testing"
	"time"
)

func TestCardDetailsAndReturn(t *testing.T) {
	c, _ := config.New("us")
	c.Portfolios[0].Positions = []config.Position{{Symbol: "VTI", Market: "us", Currency: "USD", Kind: "etf", Allocation: 1}}
	m := New(&c, "", nil, nil)
	m.width = 100
	m.bars["VTI"] = []provider.Bar{{Date: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), Close: 100}, {Date: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC), Open: 101, High: 105, Low: 99, Close: 102, Source: "fixture"}}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if m.page != detailPage {
		t.Fatal("d did not open details")
	}
	for _, s := range []string{"VTI / PRICE DETAILS", "102.00 USD", "+2.00%", "fixture", "2025-01-03", "Earliest cached history  2025-01-02"} {
		if !strings.Contains(m.View(), s) {
			t.Fatalf("details missing %q", s)
		}
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.detailOffset != 1 {
		t.Fatal("left did not pan asset history one candle")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyHome})
	if m.detailOffset != 0 {
		t.Fatal("Home did not restore latest asset history")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	if m.detailGrain != "week" {
		t.Fatal("grain did not change")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.page != holdingsPage || m.cursor != 0 {
		t.Fatal("did not return to same card")
	}
	m.cursor = 1
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if m.page != holdingsPage {
		t.Fatal("Add card opened invalid details")
	}
}
func TestDetailsLoadingAndError(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m.page = detailPage
	m.detailAsset = security{Symbol: "SPY"}
	if !strings.Contains(m.View(), "Loading price history") {
		t.Fatal("loading state missing")
	}
	m.errors["SPY"] = "offline"
	if !strings.Contains(m.View(), "offline") {
		t.Fatal("error missing")
	}
}
