package ui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/core"
)

func TestPortfolioSettingsSaveAndCancel(t *testing.T) {
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	path := filepath.Join(t.TempDir(), "config.toml")
	m := New(k, &c, path)
	m.page = analysisPage
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.page != settingsPage {
		t.Fatal("e did not open portfolio editor")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRight})
	m.settings.capital = "25000"
	m.settings.startDate = "2020-01-01"
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.page != analysisPage || m.analysis.active {
		t.Fatal("saving portfolio settings must return to Analysis selection mode")
	}
	saved, err := k.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	p := saved.Portfolio
	if p.InitialCapital != 25000 || p.StartDate != "2020-01-01" || p.Rebalance != core.RebalanceMonthly {
		t.Fatalf("wrong settings: %#v", p)
	}
	m.page = analysisPage
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m.settings.capital = "123"
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.cfg.Portfolio.InitialCapital != 25000 {
		t.Fatal("cancel changed saved settings")
	}
	if m.page != analysisPage || m.analysis.active {
		t.Fatal("cancelling portfolio settings must return to Analysis selection mode")
	}
}

func TestPortfolioSettingsInvalidCapitalStaysInEditor(t *testing.T) {
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	m := New(k, &c, filepath.Join(t.TempDir(), "config.toml"))
	m.openSettings()
	for _, value := range []string{"NaN", "0", "-1", "Inf"} {
		m.settings.capital = value
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
		if m.page != settingsPage || m.cfg.Portfolio.InitialCapital != 10000 {
			t.Fatal("invalid capital saved")
		}
	}
}

func TestSettingsOpenedFromPortfolioStillReturnsToAnalysis(t *testing.T) {
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	m := New(k, &c, filepath.Join(t.TempDir(), "config.toml"))
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if m.page != settingsPage {
		t.Fatal("s did not open settings")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.page != analysisPage {
		t.Fatal("settings should return to Analysis")
	}
}
