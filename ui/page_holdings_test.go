package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"portifolio/core"
)

func TestEmptyPortfolioHasSelectableAddCard(t *testing.T) {
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	m := New(k, &c, "")
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
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	c.Portfolio.Positions = []core.Position{{Symbol: "VTI", Market: "us", Kind: "etf", Currency: "USD", Allocation: 1}}
	m := New(k, &c, "")
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
	if m.holdings.cursor != 1 {
		t.Fatal("cannot select Add card")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.page != addPage {
		t.Fatal("Add card did not open directory")
	}
}

func TestCardKeyBindings(t *testing.T) {
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	c.Portfolio.Positions = []core.Position{{Symbol: "VTI", Market: "us", Kind: "etf", Currency: "USD", Allocation: 1}}
	for _, key := range []tea.KeyMsg{{Type: tea.KeyEnter}, {Type: tea.KeyRunes, Runes: []rune{'d'}}} {
		m := New(k, &c, "")
		m = pressKey(t, m, key)
		if m.page != detailPage || m.detail.asset.Symbol != "VTI" {
			t.Fatal("Enter/d must open asset details")
		}
	}
	m := New(k, &c, "")
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.page != weightPage || m.weight.input != "100" {
		t.Fatal("e must edit weight")
	}
}

func TestHoldingCardRowsShareLeftEdge(t *testing.T) {
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	c.Portfolio.Positions = []core.Position{{Symbol: "VTI", Market: "us", Kind: "etf", Currency: "USD", Allocation: 1}}
	m := New(k, &c, "")
	m.assets = testAssets()
	m.height = 40
	m.width = 100
	assertCardRowsAligned(t, m.View(), "VTI", "Vanguard Total Stock Market")

	c.Portfolio.Positions = append(c.Portfolio.Positions, core.Position{Symbol: "VOO", Market: "us", Kind: "etf", Currency: "USD", Allocation: 0})
	m = New(k, &c, "")
	m.assets = testAssets()
	m.height = 40
	m.width = 100
	assertCardRowsAligned(t, m.View(), "VOO", "Vanguard Total Stock Market")
}

func assertCardRowsAligned(t *testing.T, view, symbol, name string) {
	t.Helper()
	plain := ansi.Strip(view)
	lines := strings.Split(plain, "\n")
	find := func(needle string) string {
		for _, line := range lines {
			if strings.Contains(line, needle) {
				return line
			}
		}
		t.Fatalf("missing %q", needle)
		return ""
	}
	header, nameLine, lastLine := find(symbol+" · ETF"), find(name), find("Last ")
	column := func(line, needle string) int {
		index := strings.Index(line, needle)
		if index < 0 {
			t.Fatalf("%q missing from %q", needle, line)
		}
		return len([]rune(line[:index]))
	}
	want := column(header, symbol)
	if got := column(nameLine, name); got != want {
		t.Fatalf("name column %d, symbol column %d", got, want)
	}
	if got := column(lastLine, "Last"); got != want {
		t.Fatalf("price column %d, symbol column %d", got, want)
	}
}
