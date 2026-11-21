package ui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"portifolio/internal/domain"
)

func pressKey(t *testing.T, m Model, key tea.KeyMsg) Model {
	t.Helper()
	next, _ := m.Update(key)
	return next.(Model)
}

func testAssets() []domain.Asset {
	return []domain.Asset{
		{Symbol: "VTI", Name: "Vanguard Total Stock Market", Market: config.MarketUS, Kind: config.KindETF},
		{Symbol: "VOO", Name: "Vanguard S&P 500", Market: config.MarketUS, Kind: config.KindETF},
	}
}

func TestSetupOnlySelectsMarket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	m := New(nil, path, nil)
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.page != holdingsPage || m.cfg.Portfolio.Market != config.MarketCN || m.cfg.Portfolio.PriceCurrency != "CNY" {
		t.Fatal("market setup failed")
	}
	c, err := config.Load(path)
	if err != nil || len(c.Portfolio.Positions) != 0 {
		t.Fatalf("empty portfolio persistence: %v", err)
	}
}

func TestFavoriteWeightAndAnalysisFlow(t *testing.T) {
	c, _ := config.New(config.MarketUS)
	path := filepath.Join(t.TempDir(), "config.toml")
	m := New(&c, path, nil)
	m.assets = testAssets()
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.cfg.Portfolio.Positions) != 1 {
		t.Fatal("favorite was not added")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m.weight.input = "100"
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	saved, err := config.Load(path)
	if err != nil || saved.Portfolio.Positions[0].Allocation != 1 {
		t.Fatalf("weight save: %v", err)
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if m.page != analysisPage {
		t.Fatal("cannot open analysis")
	}
}

func TestInvalidWeightDoesNotMutatePortfolio(t *testing.T) {
	c, _ := config.New(config.MarketUS)
	path := filepath.Join(t.TempDir(), "config.toml")
	m := New(&c, path, nil)
	m.assets = testAssets()
	m.page = addPage
	m.toggleFavorite(m.addItems(), &m.add.cursor)
	m.page = weightPage
	m.weight.symbol = "VTI"
	m.weight.input = "NaN"
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.cfg.Portfolio.Positions[0].Allocation != 0 {
		t.Fatal("invalid value changed allocation")
	}
	m.weight.input = "25"
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.cfg.Portfolio.Positions[0].Allocation != .25 {
		t.Fatal("cannot correct invalid allocation")
	}
}

func TestWholeWeightHasNoDecimalSuffix(t *testing.T) {
	c, _ := config.New(config.MarketUS)
	c.Portfolio.Positions = []config.Position{{Symbol: "VTI", Market: config.MarketUS, Currency: "USD", Kind: config.KindETF, Allocation: .25}}
	m := New(&c, "", nil)
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.weight.input != "25" {
		t.Fatalf("expected integer default, got %q", m.weight.input)
	}
}

func TestPriceAnimationLifecycle(t *testing.T) {
	m := Model{priceLoading: map[string]bool{"SPY": true}, bars: map[string][]domain.Bar{}, errors: map[string]string{}}
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
