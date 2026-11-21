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

type Config struct {
	DefaultPortfolio string            `toml:"default_portfolio,omitempty"`
	Markets          map[string]Market `toml:"markets"`
	Portfolios       []Portfolio       `toml:"portfolios"`
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
	Market    string    `toml:"market"`
	Portfolio Portfolio `toml:"portfolio"`
}

func Builtin(market string) (string, Market, error) {
	switch market {
	case "us":
		return "USD", Market{Providers: []ProviderConfig{{Name: "yahoo", Enabled: true}, {Name: "stooq", Enabled: true}}}, nil
	case "cn":
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
	p := Portfolio{ID: "my-portfolio", Name: "My Portfolio", Market: market, BaseCurrency: currency, PriceCurrency: currency, StartDate: time.Now().AddDate(-1, 0, 0).Format("2006-01-02"), Rebalance: "never", InitialCapital: 10000}
	return Config{DefaultPortfolio: p.ID, Markets: map[string]Market{market: source}, Portfolios: []Portfolio{p}}, nil
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
	c.Portfolios[0] = disk.Portfolio
	c.DefaultPortfolio = disk.Portfolio.ID
	return c, c.Validate()
}
func (c Config) Validate() error {
	if len(c.Portfolios) != 1 {
		return fmt.Errorf("one configuration must contain exactly one portfolio")
	}
	p := c.Portfolios[0]
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
	case "never", "monthly", "quarterly", "yearly":
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
		if pos.Kind != "stock" && pos.Kind != "etf" {
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
func marketEnabled(m Market) bool {
	for _, p := range m.Providers {
		if p.Enabled {
			return true
		}
	}
	return false
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
		err = toml.NewEncoder(tmp).Encode(diskConfig{Market: c.Portfolios[0].Market, Portfolio: c.Portfolios[0]})
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
