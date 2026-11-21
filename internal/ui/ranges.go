package ui

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"time"
)

func rangeStart(end time.Time, grain string) time.Time {
	switch grain {
	case "week":
		return end.AddDate(-1, 0, 0)
	case "month":
		return end.AddDate(-5, 0, 0)
	case "year":
		return end.AddDate(-40, 0, 0)
	case "quarter":
		return end.AddDate(-15, 0, 0)
	default:
		return end.AddDate(0, -3, 0)
	}
}

// Asset history is independent of the portfolio's simulation start date.
func (m *Model) changeDetailRange(grain string) tea.Cmd {
	m.detailGrain = grain
	if m.store == nil || m.providers == nil || m.priceLoading[m.detailAsset.Symbol] {
		return nil
	}
	asset := m.detailAsset
	key := asset.Symbol + ":" + grain
	if m.rangeRequested == nil {
		m.rangeRequested = map[string]bool{}
	}
	if m.rangeRequested[key] {
		return nil
	}
	end := time.Now()
	start := rangeStart(end, grain)
	cached := m.bars[asset.Symbol]
	if len(cached) > 0 && !cached[0].Date.After(start.AddDate(0, 0, 7)) {
		return nil
	}
	wasLoading := len(m.priceLoading) > 0
	if m.priceLoading == nil {
		m.priceLoading = map[string]bool{}
	}
	m.priceLoading[asset.Symbol] = true
	m.rangeRequested[key] = true
	cfg := m.cfg
	store := m.store
	registry := m.providers
	cmd := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		pos := config.Position{Symbol: asset.Symbol, Market: cfg.Portfolios[0].Market, Currency: cfg.Portfolios[0].PriceCurrency, Kind: asset.Kind}
		bars, err := store.History(ctx, pos, start, end)
		if len(bars) == 0 || bars[0].Date.After(start.AddDate(0, 0, 7)) {
			fresh, fetchErr := registry.History(ctx, cfg.Markets, pos, start, end)
			if fetchErr == nil {
				err = store.Put(ctx, pos, fresh)
				bars = fresh
			} else {
				err = fetchErr
			}
		}
		// Read the full cache to avoid losing older data when switching back to daily.
		all, readErr := store.History(ctx, pos, time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), end)
		if readErr == nil && len(all) > 0 {
			bars = all
		}
		if len(bars) == 0 {
			bars = cached
		}
		return assetLoaded{symbol: asset.Symbol, bars: bars, err: err}
	}
	if !wasLoading {
		return tea.Batch(cmd, priceTickCmd())
	}
	return cmd
}

// Up selects a finer interval; Down selects a coarser one, with no wrap.
func stepInterval(current string, delta int) string {
	intervals := []string{"day", "week", "month", "quarter", "year"}
	index := 0
	for i, v := range intervals {
		if v == current {
			index = i
			break
		}
	}
	return intervals[max(0, min(len(intervals)-1, index+delta))]
}
