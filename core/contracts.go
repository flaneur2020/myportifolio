package core

import (
	"context"
	"time"
)

// PriceProvider fetches historical daily bars for a position.
type PriceProvider interface {
	Name() string
	Markets() []string
	History(ctx context.Context, pos Position, start, end time.Time) ([]Bar, error)
}

// DirectoryProvider fetches the searchable asset directory for a market.
type DirectoryProvider interface {
	Name() string
	Markets() []string
	Assets(ctx context.Context, market string) ([]Asset, error)
}

// CardMeta describes an analysis card's identity and interaction capabilities.
type CardMeta struct {
	ID               string
	Title            string
	SupportsInterval bool
	SupportsPan      bool
}

// CardInput supplies normalized portfolio data and view parameters to a Card.
type CardInput struct {
	Analysis AnalysisData
	Bars     map[string][]Bar
	Grain    Grain
	Offset   int
	Width    int
}

// Card is the unified contract implemented by every analysis card plugin.
type Card interface {
	Meta() CardMeta
	Evaluate(in CardInput) (CardResult, error)
}

// PriceStore caches historical price bars.
type PriceStore interface {
	History(ctx context.Context, p Position, start, end time.Time) ([]Bar, error)
	Put(ctx context.Context, p Position, bars []Bar) error
}

// DirectoryStore caches asset directories per market.
type DirectoryStore interface {
	Assets(ctx context.Context, market string) ([]Asset, time.Time, error)
	PutAssets(ctx context.Context, market string, assets []Asset) error
}

// ConfigStore persists configuration to/from disk (e.g. TOML).
// Structural decoding/encoding lives in the plugin; semantic validation lives in Kernel.
type ConfigStore interface {
	Load(path string) (Config, error)
	Save(path string, cfg Config) error
}

type DirectoryResult struct {
	Assets       []Asset
	Status       string
	NeedsRefresh bool
}
