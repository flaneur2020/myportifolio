package marketdata

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"portifolio/internal/config"
	"portifolio/internal/domain"
	"portifolio/internal/storage"
)

type stubFetcher struct {
	bars   []domain.Bar
	err    error
	called int
}

func (f *stubFetcher) History(context.Context, map[string]config.Market, config.Position, time.Time, time.Time) ([]domain.Bar, error) {
	f.called++
	return f.bars, f.err
}

type stubDirFetcher struct {
	assets []domain.Asset
	err    error
	called int
}

func (f *stubDirFetcher) Assets(context.Context, string) ([]domain.Asset, error) {
	f.called++
	return f.assets, f.err
}

func TestLoadHistoryCacheAsideAndFullCache(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	d1 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC)
	fetcher := &stubFetcher{bars: []domain.Bar{{Date: d1, Close: 100}, {Date: d2, Close: 120}}}
	svc := NewService(store, fetcher, store, nil)
	pos := config.Position{Market: config.MarketUS, Symbol: "VTI", Currency: "USD"}

	// First call misses cache and writes through.
	bars, err := svc.LoadHistory(ctx, nil, pos, d1, d2, false, nil)
	if err != nil || len(bars) != 2 || fetcher.called != 1 {
		t.Fatalf("unexpected miss result: bars=%v err=%v calls=%d", bars, err, fetcher.called)
	}

	// Second call hits cache without calling provider.
	bars, err = svc.LoadHistory(ctx, nil, pos, d1, d2, false, nil)
	if err != nil || len(bars) != 2 || fetcher.called != 1 {
		t.Fatalf("expected cache hit: bars=%v err=%v calls=%d", bars, err, fetcher.called)
	}

	// Requesting a shorter range with fullCache retains earlier cached bars.
	bars, err = svc.LoadHistory(ctx, nil, pos, d2, d2, true, nil)
	if err != nil || len(bars) != 2 || !bars[0].Date.Equal(d1) {
		t.Fatalf("fullCache dropped older bars: %#v err=%v", bars, err)
	}
}

func TestLoadDirectoryCacheFirstAndOfflineRefresh(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cached := []domain.Asset{{Symbol: "VTI", Name: "Vanguard", Market: config.MarketUS, Kind: config.KindETF}}
	if err := store.PutAssets(ctx, config.MarketUS, cached); err != nil {
		t.Fatal(err)
	}
	dir := &stubDirFetcher{err: errors.New("offline")}
	svc := NewService(store, nil, store, dir)

	res := svc.LoadDirectory(ctx, config.MarketUS, false)
	if dir.called != 0 || len(res.Assets) != 1 || res.NeedsRefresh {
		t.Fatalf("expected fresh cache hit: %#v calls=%d", res, dir.called)
	}

	res = svc.LoadDirectory(ctx, config.MarketUS, true)
	if dir.called != 1 || res.Assets != nil {
		t.Fatalf("expected offline failure without overwriting assets: %#v calls=%d", res, dir.called)
	}
}
