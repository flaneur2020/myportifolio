package marketdata

import (
	"context"
	"time"

	"portifolio/internal/config"
	"portifolio/internal/domain"
)

type PriceStore interface {
	History(ctx context.Context, p config.Position, start, end time.Time) ([]domain.Bar, error)
	Put(ctx context.Context, p config.Position, bars []domain.Bar) error
}

type PriceFetcher interface {
	History(ctx context.Context, markets map[string]config.Market, pos config.Position, start, end time.Time) ([]domain.Bar, error)
}

type DirectoryStore interface {
	Assets(ctx context.Context, market string) ([]domain.Asset, time.Time, error)
	PutAssets(ctx context.Context, market string, assets []domain.Asset) error
}

type DirectoryFetcher interface {
	Assets(ctx context.Context, market string) ([]domain.Asset, error)
}

type DirectoryResult struct {
	Assets       []domain.Asset
	Status       string
	NeedsRefresh bool
}

// Service owns cache-aside reads and writes for historical prices and asset directories.
type Service struct {
	Prices     PriceStore
	Fetcher    PriceFetcher
	DirStore   DirectoryStore
	DirFetcher DirectoryFetcher
}

func NewService(prices PriceStore, fetcher PriceFetcher, dirStore DirectoryStore, dirFetcher DirectoryFetcher) *Service {
	return &Service{
		Prices:     prices,
		Fetcher:    fetcher,
		DirStore:   dirStore,
		DirFetcher: dirFetcher,
	}
}

func (s *Service) CanLoadPrices() bool {
	return s != nil && s.Prices != nil && s.Fetcher != nil
}

func (s *Service) CanLoadDirectory() bool {
	return s != nil && s.DirStore != nil && s.DirFetcher != nil
}

// LoadHistory reads cached prices first and falls back to Fetcher when the
// cache is missing or starts more than a week after start. When fullCache is
// true, the full cached span for pos is returned so shorter ranges never drop
// previously fetched history.
func (s *Service) LoadHistory(ctx context.Context, markets map[string]config.Market, pos config.Position, start, end time.Time, fullCache bool, fallback []domain.Bar) ([]domain.Bar, error) {
	bars, err := s.Prices.History(ctx, pos, start, end)
	if len(bars) == 0 || bars[0].Date.After(start.AddDate(0, 0, 7)) {
		fresh, fetchErr := s.Fetcher.History(ctx, markets, pos, start, end)
		if fetchErr == nil && len(fresh) > 0 {
			bars = fresh
			err = s.Prices.Put(ctx, pos, bars)
		} else if fetchErr != nil {
			err = fetchErr
		}
	}
	if fullCache {
		all, readErr := s.Prices.History(ctx, pos, time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), end)
		if readErr == nil && len(all) > 0 {
			bars = all
		}
		if len(bars) == 0 {
			bars = fallback
		}
	}
	return bars, err
}

func (s *Service) LoadDirectory(ctx context.Context, market string, refresh bool) DirectoryResult {
	if !refresh {
		assets, updated, err := s.DirStore.Assets(ctx, market)
		if err == nil && len(assets) > 0 {
			return DirectoryResult{
				Assets:       assets,
				Status:       "Asset directory cached · " + updated.Format(time.RFC3339),
				NeedsRefresh: time.Since(updated) > 24*time.Hour,
			}
		}
	}
	assets, err := s.DirFetcher.Assets(ctx, market)
	if err != nil {
		return DirectoryResult{Status: "Directory refresh failed; cached assets retained: " + err.Error()}
	}
	if err = s.DirStore.PutAssets(ctx, market, assets); err != nil {
		return DirectoryResult{Status: "Directory cache write failed: " + err.Error()}
	}
	return DirectoryResult{
		Assets: assets,
		Status: "API asset directory refreshed · " + time.Now().Format(time.RFC3339),
	}
}
