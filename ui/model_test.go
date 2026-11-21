package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/core"
	"portifolio/plugins/builtin"
	tomlcfg "portifolio/plugins/config/toml"
)

func testKernel(t *testing.T) *core.Kernel {
	t.Helper()
	k := core.NewKernel()
	k.SetConfigStore(tomlcfg.New())
	if errs := builtin.Register(k, nil); len(errs) != 0 {
		t.Fatalf("builtin plugin registration failed: %v", errs)
	}
	return k
}

func pressKey(t *testing.T, m Model, key tea.KeyMsg) Model {
	t.Helper()
	next, _ := m.Update(key)
	return next.(Model)
}

func testAssets() []core.Asset {
	return []core.Asset{
		{Symbol: "VTI", Name: "Vanguard Total Stock Market", Market: "us", Kind: "etf"},
		{Symbol: "VOO", Name: "Vanguard S&P 500", Market: "us", Kind: "etf"},
	}
}

func TestSetupSelectsRegisteredMarketsDynamically(t *testing.T) {
	k := testKernel(t)
	// Add a 3rd custom market plugin dynamically — setup page adapts automatically!
	_ = k.RegisterMarket(core.MarketSpec{
		ID:          "hk",
		Name:        "Hong Kong",
		Description: "Hong Kong · HKD · Custom",
		Currency:    "HKD",
		AssetKinds:  []string{"stock", "etf"},
	}, false)

	path := filepath.Join(t.TempDir(), "config.toml")
	m := New(k, nil, path)
	if !strings.Contains(m.View(), "Hong Kong · HKD · Custom") {
		t.Fatal("setup did not render dynamically registered 3rd market")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.page != holdingsPage || m.cfg.Portfolio.Market != "cn" || m.cfg.Portfolio.PriceCurrency != "CNY" {
		t.Fatal("market setup failed")
	}
	c, err := k.LoadConfig(path)
	if err != nil || len(c.Portfolio.Positions) != 0 {
		t.Fatalf("empty portfolio persistence: %v", err)
	}
}

func TestFavoriteWeightAndAnalysisFlow(t *testing.T) {
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	path := filepath.Join(t.TempDir(), "config.toml")
	m := New(k, &c, path)
	m.assets = testAssets()
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.cfg.Portfolio.Positions) != 1 {
		t.Fatal("favorite was not added")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m.weight.input = "100"
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	saved, err := k.LoadConfig(path)
	if err != nil || saved.Portfolio.Positions[0].Allocation != 1 {
		t.Fatalf("weight save: %v", err)
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if m.page != analysisPage {
		t.Fatal("cannot open analysis")
	}
}

func TestInvalidWeightDoesNotMutatePortfolio(t *testing.T) {
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	path := filepath.Join(t.TempDir(), "config.toml")
	m := New(k, &c, path)
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
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	c.Portfolio.Positions = []core.Position{{Symbol: "VTI", Market: "us", Currency: "USD", Kind: "etf", Allocation: .25}}
	m := New(k, &c, "")
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.weight.input != "25" {
		t.Fatalf("expected integer default, got %q", m.weight.input)
	}
}

func TestPriceAnimationLifecycle(t *testing.T) {
	m := Model{priceLoading: map[string]bool{"SPY": true}, bars: map[string][]core.Bar{}, errors: map[string]string{}}
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
