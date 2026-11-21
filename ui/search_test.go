package ui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/core"
)

func TestFuzzyMatching(t *testing.T) {
	for _, q := range []string{"vti", "vgd", "total"} {
		if _, ok := fuzzyScore(q, "vti vanguard total stock market"); !ok {
			t.Fatalf("failed query %q", q)
		}
	}
	if _, ok := fuzzyScore("zzzz", "vti vanguard"); ok {
		t.Fatal("unexpected match")
	}
}

func TestMarketFuzzySearch(t *testing.T) {
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	m := New(k, &c, "")
	m.assets = testAssets()
	m.page = addPage
	m.add.query = "vgd"
	if len(m.addItems()) < 2 {
		t.Fatal("fuzzy name search failed")
	}
	for _, s := range m.addItems() {
		if s.Market != "us" {
			t.Fatal("foreign market leaked")
		}
	}
}

func TestCommonAssetsFirst(t *testing.T) {
	k := testKernel(t)
	for _, market := range []string{"us", "cn"} {
		c, _ := k.NewConfig(market)
		m := New(k, &c, "")
		m.page = addPage
		spec, _ := k.Market(market)
		names := spec.CommonSymbols
		m.assets = []core.Asset{
			{Symbol: "OTHER", Market: market},
			{Symbol: names[1], Market: market},
			{Symbol: names[0], Market: market},
		}
		got := m.addItems()
		if got[0].Symbol != names[0] || got[1].Symbol != names[1] || got[2].Symbol != "OTHER" {
			t.Fatalf("wrong common ordering: %#v", got)
		}
	}
}

func TestSearchRelevanceBeforePopularity(t *testing.T) {
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	m := New(k, &c, "")
	m.page = addPage
	m.add.query = "xyz"
	m.assets = []core.Asset{
		{Symbol: "SPY", Name: "XYZ fund", Market: "us"},
		{Symbol: "XYZ", Name: "Company", Market: "us"},
	}
	if m.addItems()[0].Symbol != "XYZ" {
		t.Fatal("popular asset outranked exact ticker")
	}
}

func TestExactTickersRankFirstInLargeDirectory(t *testing.T) {
	k := testKernel(t)
	for _, symbol := range []string{"SPY", "QQQ"} {
		t.Run(symbol, func(t *testing.T) {
			c, _ := k.NewConfig("us")
			m := New(k, &c, "")
			m.page = addPage
			for i := 0; i < 200; i++ {
				m.assets = append(m.assets, core.Asset{Symbol: fmt.Sprintf("AAA%d", i), Name: "SPY QQQ strategy fund", Market: "us", Kind: "etf"})
			}
			m.assets = append(m.assets, core.Asset{Symbol: symbol, Name: "Exact ticker fund", Market: "us", Kind: "etf"})
			m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(symbol)})
			items := m.addItems()
			if len(items) == 0 || items[0].Symbol != symbol {
				t.Fatalf("exact ticker not first for %s", symbol)
			}
			if m.add.cursor != 0 {
				t.Fatal("search did not select first result")
			}
		})
	}
}

func TestTickerPrefixRanksBeforeNameMatch(t *testing.T) {
	k := testKernel(t)
	c, _ := k.NewConfig("us")
	m := New(k, &c, "")
	m.page = addPage
	m.add.query = "spy"
	m.assets = []core.Asset{
		{Symbol: "AAA", Name: "SPY strategy", Market: "us", Kind: "etf"},
		{Symbol: "SPYG", Name: "Growth", Market: "us", Kind: "etf"},
		{Symbol: "SPY", Name: "Index", Market: "us", Kind: "etf"},
	}
	got := m.addItems()
	if len(got) != 3 || got[0].Symbol != "SPY" || got[1].Symbol != "SPYG" {
		t.Fatalf("unexpected ranking: %#v", got)
	}
}
