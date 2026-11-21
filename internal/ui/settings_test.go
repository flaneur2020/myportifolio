package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"path/filepath"
	"portifolio/internal/config"
	"testing"
)

func TestPortfolioSettingsSaveAndCancel(t *testing.T) {
	c, _ := config.New("us")
	path := filepath.Join(t.TempDir(), "config.toml")
	m := New(&c, path, nil, nil)
	m.page = analysisPage
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.page != settingsPage {
		t.Fatal("e did not open portfolio editor")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRight})
	m.settingsCapital = "25000"
	m.settingsDate = "2020-01-01"
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.page != analysisPage || m.analysisActive {
		t.Fatal("saving portfolio settings must return to Analysis selection mode")
	}
	saved, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p := saved.Portfolios[0]
	if p.InitialCapital != 25000 || p.StartDate != "2020-01-01" || p.Rebalance != "monthly" {
		t.Fatalf("wrong settings: %#v", p)
	}
	m.page = analysisPage
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m.settingsCapital = "123"
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.cfg.Portfolios[0].InitialCapital != 25000 {
		t.Fatal("cancel changed saved settings")
	}
	if m.page != analysisPage || m.analysisActive {
		t.Fatal("cancelling portfolio settings must return to Analysis selection mode")
	}
}
func TestPortfolioSettingsInvalidCapitalStaysInEditor(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, filepath.Join(t.TempDir(), "config.toml"), nil, nil)
	m.openPortfolioSettings()
	for _, value := range []string{"NaN", "0", "-1", "Inf"} {
		m.settingsCapital = value
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
		if m.page != settingsPage || m.cfg.Portfolios[0].InitialCapital != 10000 {
			t.Fatal("invalid capital saved")
		}
	}
}

func TestSettingsOpenedFromPortfolioStillReturnsToAnalysis(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, filepath.Join(t.TempDir(), "config.toml"), nil, nil)
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if m.page != settingsPage {
		t.Fatal("s did not open settings")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.page != analysisPage {
		t.Fatal("settings should return to Analysis")
	}
}
