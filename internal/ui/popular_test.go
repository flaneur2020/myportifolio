package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"strings"
	"testing"
)

func TestCommonAssetsFirst(t *testing.T) {
	for _, market := range []string{"us", "cn"} {
		c, _ := config.New(market)
		m := New(&c, "", nil, nil)
		m.page = addPage
		names := commonAssets[market]
		m.assets = []security{{Symbol: "OTHER", Market: market}, {Symbol: names[1], Market: market}, {Symbol: names[0], Market: market}}
		got := m.items()
		if got[0].Symbol != names[0] || got[1].Symbol != names[1] || got[2].Symbol != "OTHER" {
			t.Fatalf("wrong common ordering: %#v", got)
		}
	}
}
func TestSearchRelevanceBeforePopularity(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m.page = addPage
	m.query = "xyz"
	m.assets = []security{{Symbol: "SPY", Name: "XYZ fund", Market: "us"}, {Symbol: "XYZ", Name: "Company", Market: "us"}}
	if m.items()[0].Symbol != "XYZ" {
		t.Fatal("popular asset outranked exact ticker")
	}
}
func TestAddUsesSearchInputAndCards(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m.page = addPage
	m.assets = testAssets()
	m.width = 100
	view := m.View()
	for _, want := range []string{"SEARCH ASSETS", "╭", "VTI · ETF", "Enter to add"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Contains(view, "FAV") {
		t.Fatal("Add still uses table")
	}
}
func TestWholeWeightHasNoDecimalSuffix(t *testing.T) {
	c, _ := config.New("us")
	c.Portfolios[0].Positions = []config.Position{{Symbol: "VTI", Market: "us", Currency: "USD", Kind: "etf", Allocation: .25}}
	m := New(&c, "", nil, nil)
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.input != "25" {
		t.Fatalf("expected integer default, got %q", m.input)
	}
}
