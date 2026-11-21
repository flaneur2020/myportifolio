package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"strings"
	"testing"
)

func TestEmptyPortfolioHasSelectableAddCard(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	view := m.View()
	if !strings.Contains(view, "+ Add position") || strings.Contains(view, "FAV") || strings.Contains(view, "Your portfolio is empty") {
		t.Fatal("empty homepage is not a card")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.page != addPage {
		t.Fatal("Enter on empty card did not open Add")
	}
}
func TestPositionCardsAndAddNavigation(t *testing.T) {
	c, _ := config.New("us")
	c.Portfolios[0].Positions = []config.Position{{Symbol: "VTI", Market: "us", Kind: "etf", Currency: "USD", Allocation: 1}}
	m := New(&c, "", nil, nil)
	m.assets = testAssets()
	m.height = 40
	if !strings.Contains(m.View(), "VTI · ETF") || !strings.Contains(m.View(), "+ Add position") {
		t.Fatal("position and Add cards missing")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.page != detailPage {
		t.Fatal("Enter on position did not open details")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Fatal("cannot select Add card")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.page != addPage {
		t.Fatal("Add card did not open directory")
	}
}
