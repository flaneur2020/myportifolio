package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSingleMarketConfigRoundTrip(t *testing.T) {
	for _, market := range []string{MarketUS, MarketCN} {
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
		if got.Portfolio.Market != market {
			t.Fatalf("wrong portfolio market: %q", got.Portfolio.Market)
		}
	}
}

func TestPartialAllocationsAreAllowed(t *testing.T) {
	c, _ := New(MarketUS)
	c.Portfolio.Positions = []Position{{Symbol: "VTI", Market: MarketUS, Currency: "USD", Kind: KindETF, Allocation: .4}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Portfolio.Positions = append(c.Portfolio.Positions, Position{Symbol: "SPY", Market: MarketUS, Currency: "USD", Kind: KindETF, Allocation: .8})
	if err := c.Validate(); err == nil {
		t.Fatal("excess allocation accepted")
	}
}

func TestRejectForeignMarketAndMismatchedDiskMarket(t *testing.T) {
	c, _ := New(MarketUS)
	c.Portfolio.Positions = []Position{{Symbol: "510300", Market: MarketCN, Currency: "CNY", Kind: KindETF}}
	if c.Validate() == nil {
		t.Fatal("foreign market position accepted")
	}

	path := filepath.Join(t.TempDir(), "mismatch.toml")
	raw := "market = \"us\"\n[portfolio]\nid = \"p\"\nname = \"P\"\nmarket = \"cn\"\nbase_currency = \"USD\"\nprice_currency = \"USD\"\nstart_date = \"2024-01-01\"\nrebalance = \"never\"\n"
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("mismatched portfolio market accepted")
	}
}

func TestCloneIsIndependent(t *testing.T) {
	c, _ := New(MarketUS)
	c.Portfolio.Positions = []Position{{Symbol: "VTI", Market: MarketUS, Currency: "USD", Kind: KindETF, Allocation: .5}}
	c.Analysis.Order = []string{"information", "history"}
	cloned := c.Clone()
	cloned.Portfolio.Positions[0].Allocation = .9
	cloned.Analysis.Order[0] = "drawdown"
	if c.Portfolio.Positions[0].Allocation != .5 || c.Analysis.Order[0] != "information" {
		t.Fatal("clone mutated original config")
	}
}
