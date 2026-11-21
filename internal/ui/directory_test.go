package ui

import (
	"context"
	"errors"
	tea "github.com/charmbracelet/bubbletea"
	"path/filepath"
	"portifolio/internal/config"
	"portifolio/internal/provider"
	"portifolio/internal/storage"
	"strings"
	"testing"
)

func TestAddLoadingAnimationAndCancel(t *testing.T) {
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	c, _ := config.New("us")
	m := New(&c, "", store, nil)
	if m.page != holdingsPage {
		t.Fatal("portfolio is not home")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.page != addPage || !m.directoryLoading {
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
	if m.query != "vti" {
		t.Fatal("cannot search during loading")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.page != holdingsPage {
		t.Fatal("cannot cancel Add during loading")
	}
	updated, _ = m.Update(directoryLoaded{status: "offline"})
	m = updated.(Model)
	if m.directoryLoading {
		t.Fatal("animation did not stop after failure")
	}
	_, cmd = m.Update(directoryTick{})
	if cmd != nil {
		t.Fatal("animation keeps scheduling after completion")
	}
}

type failedDirectory struct{ called *bool }

func (f failedDirectory) Assets(context.Context, string) ([]provider.Asset, error) {
	*f.called = true
	return nil, errors.New("offline")
}
func TestDirectoryCacheAndOfflineRefresh(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	assets := []provider.Asset{{Symbol: "VTI", Name: "Vanguard", Market: "us", Kind: "etf"}}
	if err = store.PutAssets(ctx, "us", assets); err != nil {
		t.Fatal(err)
	}
	c, _ := config.New("us")
	m := New(&c, "", store, nil)
	called := false
	m.directory = failedDirectory{&called}
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
