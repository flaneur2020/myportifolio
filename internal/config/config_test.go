package config

import (
	"path/filepath"
	"testing"
)

func TestSingleMarketConfigRoundTrip(t *testing.T) {
	for _, market := range []string{"us", "cn"} {
		c, err := New(market)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "config.toml")
		if err = Save(path, c); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Portfolios[0].Market != market || len(got.Portfolios) != 1 {
			t.Fatal("wrong portfolio")
		}
	}
}
func TestPartialAllocationsAreAllowed(t *testing.T) {
	c, _ := New("us")
	c.Portfolios[0].Positions = []Position{{Symbol: "VTI", Market: "us", Currency: "USD", Kind: "etf", Allocation: .4}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Portfolios[0].Positions = append(c.Portfolios[0].Positions, Position{Symbol: "SPY", Market: "us", Currency: "USD", Kind: "etf", Allocation: .8})
	if err := c.Validate(); err == nil {
		t.Fatal("excess allocation accepted")
	}
}
func TestRejectMultiplePortfoliosAndMarkets(t *testing.T) {
	c, _ := New("us")
	c.Portfolios = append(c.Portfolios, c.Portfolios[0])
	if c.Validate() == nil {
		t.Fatal("multiple portfolios accepted")
	}
	c, _ = New("us")
	c.Portfolios[0].Positions = []Position{{Symbol: "510300", Market: "cn", Currency: "CNY", Kind: "etf"}}
	if c.Validate() == nil {
		t.Fatal("foreign market position accepted")
	}
}
