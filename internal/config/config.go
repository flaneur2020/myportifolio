package config

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	MarketUS = "us"
	MarketCN = "cn"

	KindStock = "stock"
	KindETF   = "etf"

	RebalanceNever     = "never"
	RebalanceMonthly   = "monthly"
	RebalanceQuarterly = "quarterly"
	RebalanceYearly    = "yearly"
)

var RebalanceOptions = []string{
	RebalanceNever,
	RebalanceMonthly,
	RebalanceQuarterly,
	RebalanceYearly,
}

type Config struct {
	Markets   map[string]Market `toml:"markets"`
	Portfolio Portfolio         `toml:"portfolio"`
	Analysis  AnalysisConfig    `toml:"analysis,omitempty"`
}

type AnalysisConfig struct {
	Order  []string `toml:"order,omitempty"`
	Hidden []string `toml:"hidden,omitempty"`
}

type Market struct {
	Providers []ProviderConfig `toml:"providers"`
}

type ProviderConfig struct {
	Name    string `toml:"name"`
	Enabled bool   `toml:"enabled"`
}

type Portfolio struct {
	ID             string     `toml:"id"`
	Name           string     `toml:"name"`
	BaseCurrency   string     `toml:"base_currency"`
	Market         string     `toml:"market,omitempty"`
	PriceCurrency  string     `toml:"price_currency,omitempty"`
	InitialCapital float64    `toml:"initial_capital,omitempty"`
	StartDate      string     `toml:"start_date"`
	Rebalance      string     `toml:"rebalance"`
	Positions      []Position `toml:"positions"`
}

type Position struct {
	Symbol     string  `toml:"symbol"`
	Market     string  `toml:"market"`
	Kind       string  `toml:"kind"`
	Allocation float64 `toml:"allocation"`
	Currency   string  `toml:"currency"`
}

// diskConfig deliberately represents one market and one portfolio per file.
type diskConfig struct {
	Market    string         `toml:"market"`
	Portfolio Portfolio      `toml:"portfolio"`
	Analysis  AnalysisConfig `toml:"analysis,omitempty"`
}

func Builtin(market string) (string, Market, error) {
	switch market {
	case MarketUS:
		return "USD", Market{Providers: []ProviderConfig{{Name: "yahoo", Enabled: true}, {Name: "stooq", Enabled: true}}}, nil
	case MarketCN:
		return "CNY", Market{Providers: []ProviderConfig{{Name: "eastmoney", Enabled: true}}}, nil
	default:
		return "", Market{}, fmt.Errorf("unsupported market %q", market)
	}
}

func New(market string) (Config, error) {
	currency, source, err := Builtin(market)
	if err != nil {
		return Config{}, err
	}
	p := Portfolio{
		ID:             "my-portfolio",
		Name:           "My Portfolio",
		Market:         market,
		BaseCurrency:   currency,
		PriceCurrency:  currency,
		StartDate:      time.Now().AddDate(-1, 0, 0).Format("2006-01-02"),
		Rebalance:      RebalanceNever,
		InitialCapital: 10000,
	}
	return Config{
		Markets:   map[string]Market{market: source},
		Portfolio: p,
	}, nil
}

func Load(path string) (Config, error) {
	var disk diskConfig
	if _, err := toml.DecodeFile(path, &disk); err != nil {
		return Config{}, err
	}
	c, err := New(disk.Market)
	if err != nil {
		return c, err
	}
	if disk.Portfolio.Market != disk.Market {
		return c, fmt.Errorf("portfolio market must match top-level market")
	}
	if disk.Portfolio.InitialCapital == 0 {
		disk.Portfolio.InitialCapital = 10000
	}
	c.Portfolio = disk.Portfolio
	c.Analysis = disk.Analysis
	return c, c.Validate()
}

func (c Config) Clone() Config {
	out := c
	if c.Markets != nil {
		out.Markets = make(map[string]Market, len(c.Markets))
		for k, m := range c.Markets {
			out.Markets[k] = Market{Providers: append([]ProviderConfig(nil), m.Providers...)}
		}
	}
	out.Portfolio.Positions = append([]Position(nil), c.Portfolio.Positions...)
	out.Analysis.Order = append([]string(nil), c.Analysis.Order...)
	out.Analysis.Hidden = append([]string(nil), c.Analysis.Hidden...)
	return out
}

func (c Config) Validate() error {
	p := c.Portfolio
	currency, _, err := Builtin(p.Market)
	if err != nil {
		return err
	}
	if p.BaseCurrency != currency || p.PriceCurrency != currency {
		return fmt.Errorf("market %s uses %s", p.Market, currency)
	}
	if p.ID == "" || strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("portfolio name and id are required")
	}
	if _, err = time.Parse("2006-01-02", p.StartDate); err != nil {
		return fmt.Errorf("start date must be YYYY-MM-DD")
	}
	if p.StartDate > time.Now().Format("2006-01-02") {
		return fmt.Errorf("start date cannot be in the future")
	}
	switch p.Rebalance {
	case RebalanceNever, RebalanceMonthly, RebalanceQuarterly, RebalanceYearly:
	default:
		return fmt.Errorf("invalid rebalance strategy")
	}
	if math.IsNaN(p.InitialCapital) || math.IsInf(p.InitialCapital, 0) || p.InitialCapital < 0 {
		return fmt.Errorf("initial capital must be a positive finite number")
	}
	total := 0.0
	seen := map[string]bool{}
	for _, pos := range p.Positions {
		if pos.Market != p.Market || pos.Currency != currency {
			return fmt.Errorf("position %s must use portfolio market and currency", pos.Symbol)
		}
		if pos.Symbol == "" || seen[pos.Symbol] {
			return fmt.Errorf("empty or duplicate symbol %q", pos.Symbol)
		}
		seen[pos.Symbol] = true
		if pos.Kind != KindStock && pos.Kind != KindETF {
			return fmt.Errorf("invalid asset type")
		}
		if math.IsNaN(pos.Allocation) || math.IsInf(pos.Allocation, 0) || pos.Allocation < 0 || pos.Allocation > 1 {
			return fmt.Errorf("invalid allocation")
		}
		total += pos.Allocation
	}
	if total > 1.001 {
		return fmt.Errorf("allocations exceed 100%%")
	}
	return nil
}

func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "config-*.toml")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0600); err == nil {
		err = toml.NewEncoder(tmp).Encode(diskConfig{
			Market:    c.Portfolio.Market,
			Portfolio: c.Portfolio,
			Analysis:  c.Analysis,
		})
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
