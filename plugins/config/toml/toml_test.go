package tomlcfg

import (
	"os"
	"path/filepath"
	"testing"

	"portifolio/core"
)

func testKernel() *core.Kernel {
	k := core.NewKernel()
	k.SetConfigStore(New())
	_ = k.RegisterMarket(core.MarketSpec{ID: "us", Currency: "USD", AssetKinds: []string{"stock", "etf"}}, false)
	_ = k.RegisterMarket(core.MarketSpec{ID: "cn", Currency: "CNY", AssetKinds: []string{"stock", "etf"}}, false)
	return k
}

func TestSingleMarketConfigRoundTrip(t *testing.T) {
	k := testKernel()
	for _, market := range []string{"us", "cn"} {
		c, err := k.NewConfig(market)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "config.toml")
		if err = k.SaveConfig(path, c); err != nil {
			t.Fatal(err)
		}
		got, err := k.LoadConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Portfolio.Market != market {
			t.Fatalf("wrong portfolio market: %q", got.Portfolio.Market)
		}
	}
}

func TestPartialAllocationsAndRejection(t *testing.T) {
	k := testKernel()
	c, _ := k.NewConfig("us")
	c.Portfolio.Positions = []core.Position{{Symbol: "VTI", Market: "us", Currency: "USD", Kind: "etf", Allocation: .4}}
	if err := k.ValidateConfig(c); err != nil {
		t.Fatal(err)
	}
	c.Portfolio.Positions = append(c.Portfolio.Positions, core.Position{Symbol: "SPY", Market: "us", Currency: "USD", Kind: "etf", Allocation: .8})
	if err := k.ValidateConfig(c); err == nil {
		t.Fatal("excess allocation accepted")
	}

	c2, _ := k.NewConfig("us")
	c2.Portfolio.Positions = []core.Position{{Symbol: "510300", Market: "cn", Currency: "CNY", Kind: "etf"}}
	if k.ValidateConfig(c2) == nil {
		t.Fatal("foreign market position accepted")
	}

	path := filepath.Join(t.TempDir(), "mismatch.toml")
	raw := "market = \"us\"\n[portfolio]\nid = \"p\"\nname = \"P\"\nmarket = \"cn\"\nbase_currency = \"USD\"\nprice_currency = \"USD\"\nstart_date = \"2024-01-01\"\nrebalance = \"never\"\n"
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := k.LoadConfig(path); err == nil {
		t.Fatal("mismatched portfolio market accepted")
	}
}

func TestCloneIsIndependent(t *testing.T) {
	k := testKernel()
	c, _ := k.NewConfig("us")
	c.Portfolio.Positions = []core.Position{{Symbol: "VTI", Market: "us", Currency: "USD", Kind: "etf", Allocation: .5}}
	c.Analysis.Order = []string{"information", "history"}
	cloned := c.Clone()
	cloned.Portfolio.Positions[0].Allocation = .9
	cloned.Analysis.Order[0] = "drawdown"
	if c.Portfolio.Positions[0].Allocation != .5 || c.Analysis.Order[0] != "information" {
		t.Fatal("clone mutated original config")
	}
}

func TestExistingMyPortfolioTOMLLoadsCleanly(t *testing.T) {
	k := testKernel()
	cfg, err := k.LoadConfig(filepath.Join("..", "..", "..", "my-portfolio.toml"))
	if err != nil {
		t.Fatalf("existing my-portfolio.toml failed to load: %v", err)
	}
	if cfg.Portfolio.Market != "us" || len(cfg.Portfolio.Positions) != 4 || len(cfg.Analysis.Order) != 7 {
		t.Fatalf("unexpected loaded config: %#v", cfg)
	}
}
