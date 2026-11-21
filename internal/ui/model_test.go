package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"path/filepath"
	"portifolio/internal/config"
	"testing"
)

func pressKey(t *testing.T, m Model, key tea.KeyMsg) Model {
	t.Helper()
	next, _ := m.Update(key)
	return next.(Model)
}
func TestSetupOnlySelectsMarket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	m := New(nil, path, nil, nil)
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.page != holdingsPage || m.cfg.Portfolios[0].Market != "cn" || m.cfg.Portfolios[0].PriceCurrency != "CNY" {
		t.Fatal("market setup failed")
	}
	c, err := config.Load(path)
	if err != nil || len(c.Portfolios[0].Positions) != 0 {
		t.Fatalf("empty portfolio persistence: %v", err)
	}
}
func TestFavoriteWeightAndAnalysisFlow(t *testing.T) {
	c, _ := config.New("us")
	path := filepath.Join(t.TempDir(), "config.toml")
	m := New(&c, path, nil, nil)
	m.assets = testAssets()
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.cfg.Portfolios[0].Positions) != 1 {
		t.Fatal("favorite was not added")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m.input = "100"
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	saved, err := config.Load(path)
	if err != nil || saved.Portfolios[0].Positions[0].Allocation != 1 {
		t.Fatalf("weight save: %v", err)
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if m.page != analysisPage {
		t.Fatal("cannot open analysis")
	}
}
func TestMarketFuzzySearch(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m.assets = testAssets()
	m.page = addPage
	m.query = "vgd"
	if len(m.items()) < 2 {
		t.Fatal("fuzzy name search failed")
	}
	for _, s := range m.items() {
		if s.Market != "us" {
			t.Fatal("foreign market leaked")
		}
	}
}
func TestInvalidWeightDoesNotMutatePortfolio(t *testing.T) {
	c, _ := config.New("us")
	path := filepath.Join(t.TempDir(), "config.toml")
	m := New(&c, path, nil, nil)
	m.assets = testAssets()
	m.page = addPage
	m.toggleFavorite()
	m.page = weightPage
	m.weightSymbol = "VTI"
	m.input = "NaN"
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.cfg.Portfolios[0].Positions[0].Allocation != 0 {
		t.Fatal("invalid value changed allocation")
	}
	m.input = "25"
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.cfg.Portfolios[0].Positions[0].Allocation != .25 {
		t.Fatal("cannot correct invalid allocation")
	}
}

func testAssets() []security {
	return []security{{"VTI", "Vanguard Total Stock Market", "us", "etf"}, {"VOO", "Vanguard S&P 500", "us", "etf"}}
}
