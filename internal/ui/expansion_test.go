package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"testing"
)

func TestCardKeyBindings(t *testing.T) {
	c, _ := config.New("us")
	c.Portfolios[0].Positions = []config.Position{{Symbol: "VTI", Market: "us", Kind: "etf", Currency: "USD", Allocation: 1}}
	for _, key := range []tea.KeyMsg{{Type: tea.KeyEnter}, {Type: tea.KeyRunes, Runes: []rune{'d'}}} {
		m := New(&c, "", nil, nil)
		m = pressKey(t, m, key)
		if m.page != detailPage || m.detailAsset.Symbol != "VTI" {
			t.Fatal("Enter/d must open asset details")
		}
	}
	m := New(&c, "", nil, nil)
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.page != weightPage || m.input != "100" {
		t.Fatal("e must edit weight")
	}
}
