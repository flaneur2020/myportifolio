package ui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"portifolio/internal/marketdata"
	"portifolio/internal/storage"
)

type failedDirectory struct{ called *bool }

func (f failedDirectory) Assets(context.Context, string) ([]domain.Asset, error) {
	*f.called = true
	return nil, errors.New("offline")
}

func TestAddUsesSearchInputAndCards(t *testing.T) {
	c, _ := config.New(config.MarketUS)
	m := New(&c, "", nil)
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

func TestHistoryOnlyInsideSearchCards(t *testing.T) {
	c, _ := config.New(config.MarketUS)
	c.Portfolio.Positions = []config.Position{{Symbol: "VTI", Market: config.MarketUS, Kind: config.KindETF, Currency: "USD", Allocation: 1}}
	m := New(&c, "", nil)
	m.width = 100
	m.height = 60
	m.assets = testAssets()
	m.bars["VTI"] = []domain.Bar{
		{Date: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), Open: 100, High: 105, Low: 95, Close: 104},
		{Date: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC), Open: 104, High: 106, Low: 98, Close: 99},
	}
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
	m := Model{priceLoading: map[string]bool{"SPY": true}, bars: map[string][]domain.Bar{}, errors: map[string]string{}}
	if !strings.Contains(m.cardHistory("SPY", 80), "Loading price history") {
		t.Fatal("loading state missing")
	}
	delete(m.priceLoading, "SPY")
	m.errors["SPY"] = "offline"
	if !strings.Contains(m.cardHistory("SPY", 80), "History unavailable") {
		t.Fatal("failure state missing")
	}
}

func TestAddLoadingAnimationAndCancel(t *testing.T) {
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	called := false
	svc := marketdata.NewService(store, nil, store, failedDirectory{&called})
	c, _ := config.New(config.MarketUS)
	m := New(&c, "", svc)
	if m.page != holdingsPage {
		t.Fatal("portfolio is not home")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.page != addPage || !m.dirLoading {
		t.Fatal("Add did not start loading")
	}
	before := m.directoryProgress(80)
	updated, cmd := m.Update(directoryTick{})
	m = updated.(Model)
	if cmd == nil || before == m.directoryProgress(80) {
		t.Fatal("loading bar did not animate")
	}
	if !strings.Contains(m.View(), "Loading asset directory") {
		t.Fatal("loading bar missing from Add")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("vti")})
	if m.add.query != "vti" {
		t.Fatal("cannot search during loading")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.page != holdingsPage {
		t.Fatal("cannot cancel Add during loading")
	}
	updated, _ = m.Update(directoryLoaded{status: "offline"})
	m = updated.(Model)
	if m.dirLoading {
		t.Fatal("animation did not stop after failure")
	}
	_, cmd = m.Update(directoryTick{})
	if cmd != nil {
		t.Fatal("animation keeps scheduling after completion")
	}
}

func TestDirectoryCacheAndOfflineRefresh(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	assets := []domain.Asset{{Symbol: "VTI", Name: "Vanguard", Market: config.MarketUS, Kind: config.KindETF}}
	if err = store.PutAssets(ctx, config.MarketUS, assets); err != nil {
		t.Fatal(err)
	}
	called := false
	svc := marketdata.NewService(store, nil, store, failedDirectory{&called})
	c, _ := config.New(config.MarketUS)
	m := New(&c, "", svc)
	cached := m.loadDirectory(false)().(directoryLoaded)
	if called || len(cached.assets) != 1 {
		t.Fatal("startup did not use cache first")
	}
	updated, _ := m.Update(cached)
	m = updated.(Model)
	failure := m.loadDirectory(true)().(directoryLoaded)
	updated, _ = m.Update(failure)
	m = updated.(Model)
	if !called || len(m.assets) != 1 || m.assets[0].Symbol != "VTI" {
		t.Fatal("offline refresh lost cached assets")
	}
}
