package tomlcfg

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"portifolio/core"
)

// Store is the TOML configuration plugin implementing core.ConfigStore.
type Store struct{}

func New() Store { return Store{} }

type diskConfig struct {
	Market    string       `toml:"market"`
	Theme     string       `toml:"theme,omitempty"`
	Portfolio diskPortfolio `toml:"portfolio"`
	Analysis  diskAnalysis `toml:"analysis,omitempty"`
}

type diskAnalysis struct {
	Order  []string `toml:"order,omitempty"`
	Hidden []string `toml:"hidden,omitempty"`
}

type diskPortfolio struct {
	ID             string         `toml:"id"`
	Name           string         `toml:"name"`
	BaseCurrency   string         `toml:"base_currency"`
	Market         string         `toml:"market,omitempty"`
	PriceCurrency  string         `toml:"price_currency,omitempty"`
	InitialCapital float64        `toml:"initial_capital,omitempty"`
	StartDate      string         `toml:"start_date"`
	Rebalance      string         `toml:"rebalance"`
	Positions      []diskPosition `toml:"positions"`
}

type diskPosition struct {
	Symbol     string  `toml:"symbol"`
	Market     string  `toml:"market"`
	Kind       string  `toml:"kind"`
	Allocation float64 `toml:"allocation"`
	Currency   string  `toml:"currency"`
}

func (Store) Load(path string) (core.Config, error) {
	var disk diskConfig
	if _, err := toml.DecodeFile(path, &disk); err != nil {
		return core.Config{}, err
	}
	positions := make([]core.Position, 0, len(disk.Portfolio.Positions))
	for _, p := range disk.Portfolio.Positions {
		positions = append(positions, core.Position{
			Symbol:     p.Symbol,
			Market:     p.Market,
			Kind:       p.Kind,
			Allocation: p.Allocation,
			Currency:   p.Currency,
		})
	}
	return core.Config{
		Market: disk.Market,
		Theme:  disk.Theme,
		Portfolio: core.Portfolio{
			ID:             disk.Portfolio.ID,
			Name:           disk.Portfolio.Name,
			BaseCurrency:   disk.Portfolio.BaseCurrency,
			Market:         disk.Portfolio.Market,
			PriceCurrency:  disk.Portfolio.PriceCurrency,
			InitialCapital: disk.Portfolio.InitialCapital,
			StartDate:      disk.Portfolio.StartDate,
			Rebalance:      disk.Portfolio.Rebalance,
			Positions:      positions,
		},
		Analysis: core.AnalysisConfig{
			Order:  append([]string(nil), disk.Analysis.Order...),
			Hidden: append([]string(nil), disk.Analysis.Hidden...),
		},
	}, nil
}

func (Store) Save(path string, c core.Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "config-*.toml")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)

	positions := make([]diskPosition, 0, len(c.Portfolio.Positions))
	for _, p := range c.Portfolio.Positions {
		positions = append(positions, diskPosition{
			Symbol:     p.Symbol,
			Market:     p.Market,
			Kind:       p.Kind,
			Allocation: p.Allocation,
			Currency:   p.Currency,
		})
	}
	disk := diskConfig{
		Market: c.Portfolio.Market,
		Theme:  c.Theme,
		Portfolio: diskPortfolio{
			ID:             c.Portfolio.ID,
			Name:           c.Portfolio.Name,
			BaseCurrency:   c.Portfolio.BaseCurrency,
			Market:         c.Portfolio.Market,
			PriceCurrency:  c.Portfolio.PriceCurrency,
			InitialCapital: c.Portfolio.InitialCapital,
			StartDate:      c.Portfolio.StartDate,
			Rebalance:      c.Portfolio.Rebalance,
			Positions:      positions,
		},
		Analysis: diskAnalysis{
			Order:  append([]string(nil), c.Analysis.Order...),
			Hidden: append([]string(nil), c.Analysis.Hidden...),
		},
	}

	if err = tmp.Chmod(0600); err == nil {
		err = toml.NewEncoder(tmp).Encode(disk)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
