package provider

import (
	"context"
	"fmt"
	"time"

	"portifolio/internal/config"
	"portifolio/internal/domain"
)

type Provider interface {
	Name() string
	Markets() []string
	History(context.Context, config.Position, time.Time, time.Time) ([]domain.Bar, error)
}

type Registry struct{ items map[string]Provider }

func NewRegistry() *Registry            { return &Registry{items: map[string]Provider{}} }
func (r *Registry) Register(p Provider) { r.items[p.Name()] = p }

func (r *Registry) History(ctx context.Context, markets map[string]config.Market, pos config.Position, start, end time.Time) ([]domain.Bar, error) {
	m, ok := markets[pos.Market]
	if !ok {
		return nil, fmt.Errorf("no providers configured for market %s", pos.Market)
	}
	var last error
	for _, entry := range m.Providers {
		if !entry.Enabled {
			continue
		}
		p, ok := r.items[entry.Name]
		if !ok {
			last = fmt.Errorf("provider %s is not registered", entry.Name)
			continue
		}
		bars, err := p.History(ctx, pos, start, end)
		if err == nil && len(bars) > 0 {
			return bars, nil
		}
		if err != nil {
			last = err
		} else {
			last = fmt.Errorf("provider %s returned no prices", entry.Name)
		}
	}
	if last == nil {
		last = fmt.Errorf("no enabled provider for market %s", pos.Market)
	}
	return nil, last
}
