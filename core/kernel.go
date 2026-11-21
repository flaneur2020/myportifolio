package core

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// Kernel is the micro-core container. It owns no concrete markets, providers,
// cards, themes, or storage drivers; everything is registered via plugins.
type Kernel struct {
	mu sync.RWMutex

	marketOrder []string
	markets     map[string]MarketSpec

	priceProviders map[string]PriceProvider
	dirProviders   map[string]DirectoryProvider

	cardOrder []string
	cards     map[string]Card

	themeOrder []string
	themes     map[string]ThemeSpec

	priceStore  PriceStore
	dirStore    DirectoryStore
	configStore ConfigStore

	pluginErrors []string
	closers      []func() error
}

func NewKernel() *Kernel {
	return &Kernel{
		markets:        map[string]MarketSpec{},
		priceProviders: map[string]PriceProvider{},
		dirProviders:   map[string]DirectoryProvider{},
		cards:          map[string]Card{},
		themes:         map[string]ThemeSpec{},
	}
}

func (k *Kernel) SetPriceStore(s PriceStore)        { k.mu.Lock(); k.priceStore = s; k.mu.Unlock() }
func (k *Kernel) SetDirectoryStore(s DirectoryStore) { k.mu.Lock(); k.dirStore = s; k.mu.Unlock() }
func (k *Kernel) SetConfigStore(s ConfigStore)      { k.mu.Lock(); k.configStore = s; k.mu.Unlock() }

func (k *Kernel) OnClose(fn func() error) {
	if fn == nil {
		return
	}
	k.mu.Lock()
	k.closers = append(k.closers, fn)
	k.mu.Unlock()
}

func (k *Kernel) Close() error {
	k.mu.Lock()
	closers := append([]func() error(nil), k.closers...)
	k.closers = nil
	k.mu.Unlock()
	var firstErr error
	for i := len(closers) - 1; i >= 0; i-- {
		if err := closers[i](); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (k *Kernel) AddPluginError(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	k.mu.Lock()
	k.pluginErrors = append(k.pluginErrors, msg)
	k.mu.Unlock()
}

func (k *Kernel) PluginErrors() []string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return append([]string(nil), k.pluginErrors...)
}

func (k *Kernel) RegisterMarket(spec MarketSpec, override bool) error {
	if !ValidID(spec.ID) {
		return fmt.Errorf("invalid market id %q", spec.ID)
	}
	if strings.TrimSpace(spec.Currency) == "" {
		return fmt.Errorf("market %q requires currency", spec.ID)
	}
	if strings.TrimSpace(spec.Name) == "" {
		spec.Name = strings.ToUpper(spec.ID)
	}
	if strings.TrimSpace(spec.Description) == "" {
		spec.Description = spec.Name + " · " + spec.Currency
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, exists := k.markets[spec.ID]; exists {
		if !override {
			return fmt.Errorf("duplicate market id %q", spec.ID)
		}
	} else {
		k.marketOrder = append(k.marketOrder, spec.ID)
	}
	k.markets[spec.ID] = spec.Clone()
	return nil
}

func (k *Kernel) Markets() []MarketSpec {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]MarketSpec, 0, len(k.marketOrder))
	for _, id := range k.marketOrder {
		out = append(out, k.markets[id].Clone())
	}
	return out
}

func (k *Kernel) Market(id string) (MarketSpec, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	m, ok := k.markets[id]
	if !ok {
		return MarketSpec{}, false
	}
	return m.Clone(), true
}

func (k *Kernel) RegisterPriceProvider(p PriceProvider, override bool) error {
	if p == nil || !ValidID(p.Name()) {
		return fmt.Errorf("invalid price provider")
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, exists := k.priceProviders[p.Name()]; exists && !override {
		return fmt.Errorf("duplicate price provider %q", p.Name())
	}
	k.priceProviders[p.Name()] = p
	return nil
}

func (k *Kernel) RegisterDirectoryProvider(p DirectoryProvider, override bool) error {
	if p == nil || !ValidID(p.Name()) {
		return fmt.Errorf("invalid directory provider")
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, exists := k.dirProviders[p.Name()]; exists && !override {
		return fmt.Errorf("duplicate directory provider %q", p.Name())
	}
	k.dirProviders[p.Name()] = p
	return nil
}

func (k *Kernel) RegisterCard(c Card, override bool) error {
	if c == nil {
		return fmt.Errorf("nil card")
	}
	meta := c.Meta()
	if !ValidID(meta.ID) {
		return fmt.Errorf("invalid card id %q", meta.ID)
	}
	if strings.TrimSpace(meta.Title) == "" {
		return fmt.Errorf("card %q requires title", meta.ID)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, exists := k.cards[meta.ID]; exists {
		if !override {
			return fmt.Errorf("duplicate card id %q", meta.ID)
		}
	} else {
		k.cardOrder = append(k.cardOrder, meta.ID)
	}
	k.cards[meta.ID] = c
	return nil
}

func (k *Kernel) Cards() []Card {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]Card, 0, len(k.cardOrder))
	for _, id := range k.cardOrder {
		out = append(out, k.cards[id])
	}
	return out
}

func (k *Kernel) Card(id string) (Card, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	c, ok := k.cards[id]
	return c, ok
}

func (k *Kernel) RegisterTheme(spec ThemeSpec, override bool) error {
	if !ValidID(spec.ID) {
		return fmt.Errorf("invalid theme id %q", spec.ID)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, exists := k.themes[spec.ID]; exists {
		if !override {
			return fmt.Errorf("duplicate theme id %q", spec.ID)
		}
	} else {
		k.themeOrder = append(k.themeOrder, spec.ID)
	}
	k.themes[spec.ID] = spec.Clone()
	return nil
}

func (k *Kernel) Theme(id string) (ThemeSpec, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	t, ok := k.themes[id]
	if !ok {
		return ThemeSpec{}, false
	}
	return t.Clone(), true
}

func (k *Kernel) ActiveTheme(preferred string) ThemeSpec {
	if preferred != "" {
		if t, ok := k.Theme(preferred); ok {
			return t
		}
	}
	if t, ok := k.Theme("default"); ok {
		return t
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	if len(k.themeOrder) > 0 {
		return k.themes[k.themeOrder[0]].Clone()
	}
	return ThemeSpec{}
}

// NewConfig builds a valid default configuration for a registered market.
func (k *Kernel) NewConfig(marketID string) (Config, error) {
	market, ok := k.Market(marketID)
	if !ok {
		return Config{}, fmt.Errorf("unsupported market %q", marketID)
	}
	p := Portfolio{
		ID:             "my-portfolio",
		Name:           "My Portfolio",
		Market:         market.ID,
		BaseCurrency:   market.Currency,
		PriceCurrency:  market.Currency,
		StartDate:      time.Now().AddDate(-1, 0, 0).Format("2006-01-02"),
		Rebalance:      RebalanceNever,
		InitialCapital: 10000,
	}
	return Config{
		Market:    market.ID,
		Portfolio: p,
	}, nil
}

// ValidateConfig validates a Config against the markets currently registered in Kernel.
func (k *Kernel) ValidateConfig(c Config) error {
	p := c.Portfolio
	if c.Market != "" && p.Market != c.Market {
		return fmt.Errorf("portfolio market must match top-level market")
	}
	market, ok := k.Market(p.Market)
	if !ok {
		return fmt.Errorf("unsupported market %q", p.Market)
	}
	if p.BaseCurrency != market.Currency || p.PriceCurrency != market.Currency {
		return fmt.Errorf("market %s uses %s", p.Market, market.Currency)
	}
	if p.ID == "" || strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("portfolio name and id are required")
	}
	if _, err := time.Parse("2006-01-02", p.StartDate); err != nil {
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
		if pos.Market != p.Market || pos.Currency != market.Currency {
			return fmt.Errorf("position %s must use portfolio market and currency", pos.Symbol)
		}
		if pos.Symbol == "" || seen[pos.Symbol] {
			return fmt.Errorf("empty or duplicate symbol %q", pos.Symbol)
		}
		seen[pos.Symbol] = true
		if !market.AllowsKind(pos.Kind) {
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

func (k *Kernel) LoadConfig(path string) (Config, error) {
	k.mu.RLock()
	store := k.configStore
	k.mu.RUnlock()
	if store == nil {
		return Config{}, fmt.Errorf("no config store registered")
	}
	cfg, err := store.Load(path)
	if err != nil {
		return Config{}, err
	}
	if cfg.Market != "" && cfg.Portfolio.Market != cfg.Market {
		return Config{}, fmt.Errorf("portfolio market must match top-level market")
	}
	if cfg.Market == "" {
		cfg.Market = cfg.Portfolio.Market
	}
	if cfg.Portfolio.InitialCapital == 0 {
		cfg.Portfolio.InitialCapital = 10000
	}
	return cfg, k.ValidateConfig(cfg)
}

func (k *Kernel) SaveConfig(path string, cfg Config) error {
	if cfg.Market == "" {
		cfg.Market = cfg.Portfolio.Market
	}
	if err := k.ValidateConfig(cfg); err != nil {
		return err
	}
	k.mu.RLock()
	store := k.configStore
	k.mu.RUnlock()
	if store == nil {
		return fmt.Errorf("no config store registered")
	}
	return store.Save(path, cfg)
}

func (k *Kernel) CanLoadPrices() bool {
	if k == nil {
		return false
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.priceStore != nil && len(k.priceProviders) > 0
}

func (k *Kernel) CanLoadDirectory() bool {
	if k == nil {
		return false
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.dirStore != nil && len(k.dirProviders) > 0
}

// FetchHistory walks the registered market's ordered price provider fallback chain.
func (k *Kernel) FetchHistory(ctx context.Context, pos Position, start, end time.Time) ([]Bar, error) {
	market, ok := k.Market(pos.Market)
	if !ok {
		return nil, fmt.Errorf("no providers configured for market %s", pos.Market)
	}
	k.mu.RLock()
	providers := make([]PriceProvider, 0, len(market.PriceProviders))
	var missing []string
	for _, name := range market.PriceProviders {
		if p, ok := k.priceProviders[name]; ok {
			providers = append(providers, p)
		} else {
			missing = append(missing, name)
		}
	}
	k.mu.RUnlock()

	var last error
	for _, p := range providers {
		bars, err := p.History(ctx, pos, start, end)
		if err == nil && len(bars) > 0 {
			return bars, nil
		}
		if err != nil {
			last = err
		} else {
			last = fmt.Errorf("provider %s returned no prices", p.Name())
		}
	}
	if last == nil && len(missing) > 0 {
		last = fmt.Errorf("provider %s is not registered", missing[0])
	}
	if last == nil {
		last = fmt.Errorf("no enabled provider for market %s", pos.Market)
	}
	return nil, last
}

// LoadHistory reads cached prices first and falls back to registered providers.
func (k *Kernel) LoadHistory(ctx context.Context, pos Position, start, end time.Time, fullCache bool, fallback []Bar) ([]Bar, error) {
	k.mu.RLock()
	store := k.priceStore
	k.mu.RUnlock()
	if store == nil {
		return fallback, fmt.Errorf("no price store registered")
	}
	bars, err := store.History(ctx, pos, start, end)
	if len(bars) == 0 || bars[0].Date.After(start.AddDate(0, 0, 7)) {
		fresh, fetchErr := k.FetchHistory(ctx, pos, start, end)
		if fetchErr == nil && len(fresh) > 0 {
			bars = fresh
			err = store.Put(ctx, pos, bars)
		} else if fetchErr != nil {
			err = fetchErr
		}
	}
	if fullCache {
		all, readErr := store.History(ctx, pos, time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), end)
		if readErr == nil && len(all) > 0 {
			bars = all
		}
		if len(bars) == 0 {
			bars = fallback
		}
	}
	return bars, err
}

// FetchDirectory fetches a market's asset directory from its registered provider.
func (k *Kernel) FetchDirectory(ctx context.Context, marketID string) ([]Asset, error) {
	market, ok := k.Market(marketID)
	if !ok {
		return nil, fmt.Errorf("unsupported market %q", marketID)
	}
	k.mu.RLock()
	provider := k.dirProviders[market.DirectoryProvider]
	k.mu.RUnlock()
	if provider == nil {
		return nil, fmt.Errorf("directory provider %q is not registered", market.DirectoryProvider)
	}
	return provider.Assets(ctx, marketID)
}

func (k *Kernel) LoadDirectory(ctx context.Context, marketID string, refresh bool) DirectoryResult {
	k.mu.RLock()
	store := k.dirStore
	k.mu.RUnlock()
	if store == nil {
		return DirectoryResult{Status: "Directory cache unavailable"}
	}
	if !refresh {
		assets, updated, err := store.Assets(ctx, marketID)
		if err == nil && len(assets) > 0 {
			return DirectoryResult{
				Assets:       assets,
				Status:       "Asset directory cached · " + updated.Format(time.RFC3339),
				NeedsRefresh: time.Since(updated) > 24*time.Hour,
			}
		}
	}
	assets, err := k.FetchDirectory(ctx, marketID)
	if err != nil {
		return DirectoryResult{Status: "Directory refresh failed; cached assets retained: " + err.Error()}
	}
	if err = store.PutAssets(ctx, marketID, assets); err != nil {
		return DirectoryResult{Status: "Directory cache write failed: " + err.Error()}
	}
	return DirectoryResult{
		Assets: assets,
		Status: "API asset directory refreshed · " + time.Now().Format(time.RFC3339),
	}
}
