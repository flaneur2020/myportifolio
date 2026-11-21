package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"testing"
)

func TestExactTickersRankFirstInLargeDirectory(t *testing.T) {
	for _, symbol := range []string{"SPY", "QQQ"} {
		t.Run(symbol, func(t *testing.T) {
			c, _ := config.New("us")
			m := New(&c, "", nil, nil)
			m.page = addPage
			for i := 0; i < 200; i++ {
				m.assets = append(m.assets, security{fmt.Sprintf("AAA%d", i), "SPY QQQ strategy fund", "us", "etf"})
			}
			m.assets = append(m.assets, security{symbol, "Exact ticker fund", "us", "etf"})
			m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(symbol)})
			items := m.items()
			if len(items) == 0 || items[0].Symbol != symbol {
				t.Fatalf("exact ticker not first for %s", symbol)
			}
			if m.cursor != 0 {
				t.Fatal("search did not select first result")
			}
		})
	}
}
func TestTickerPrefixRanksBeforeNameMatch(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m.page = addPage
	m.query = "spy"
	m.assets = []security{{"AAA", "SPY strategy", "us", "etf"}, {"SPYG", "Growth", "us", "etf"}, {"SPY", "Index", "us", "etf"}}
	got := m.items()
	if len(got) != 3 || got[0].Symbol != "SPY" || got[1].Symbol != "SPYG" {
		t.Fatalf("unexpected ranking: %#v", got)
	}
}
