package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"portifolio/core"
)

func TestSQLitePriceRoundTrip(t *testing.T) {
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := core.Position{Market: "us", Symbol: "SPY"}
	d := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	if err = s.Put(context.Background(), p, []core.Bar{{Date: d, Close: 470, AdjustedClose: 468, Currency: "USD", Source: "mock"}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.History(context.Background(), p, d, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].AdjustedClose != 468 || got[0].Source != "mock" {
		t.Fatalf("unexpected cached bars: %#v", got)
	}
}

func TestDirectorySnapshotAtomicAndCustomKindsAllowed(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	us := []core.Asset{{Symbol: "VTI", Name: "Vanguard", Market: "us", Kind: "etf"}}
	if err = s.PutAssets(ctx, "us", us); err != nil {
		t.Fatal(err)
	}
	// Custom market & asset kind from plugin (e.g. crypto/spot) works without modifying SQLite!
	crypto := []core.Asset{{Symbol: "BTC", Name: "Bitcoin", Market: "crypto", Kind: "spot"}}
	if err = s.PutAssets(ctx, "crypto", crypto); err != nil {
		t.Fatal(err)
	}
	if err = s.PutAssets(ctx, "us", nil); err == nil {
		t.Fatal("empty snapshot accepted")
	}
	duplicate := append(append([]core.Asset{}, us...), us...)
	if err = s.PutAssets(ctx, "us", duplicate); err == nil {
		t.Fatal("duplicate snapshot accepted")
	}
	got, stamp, err := s.Assets(ctx, "us")
	if err != nil || len(got) != 1 || got[0].Symbol != "VTI" || stamp.IsZero() {
		t.Fatalf("snapshot lost: %#v %v", got, err)
	}
	got, _, err = s.Assets(ctx, "crypto")
	if err != nil || len(got) != 1 || got[0].Kind != "spot" {
		t.Fatalf("custom market lost: %#v %v", got, err)
	}
}

type stubFetcher struct {
	bars   []core.Bar
	err    error
	called int
}

func (f *stubFetcher) Name() string      { return "stub" }
func (f *stubFetcher) Markets() []string { return []string{"us"} }
func (f *stubFetcher) History(context.Context, core.Position, time.Time, time.Time) ([]core.Bar, error) {
	f.called++
	return f.bars, f.err
}

type stubDir struct {
	assets []core.Asset
	err    error
	called int
}

func (f *stubDir) Name() string      { return "stub_dir" }
func (f *stubDir) Markets() []string { return []string{"us"} }
func (f *stubDir) Assets(context.Context, string) ([]core.Asset, error) {
	f.called++
	return f.assets, f.err
}

func TestKernelCacheAsideWithSQLite(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	d1 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC)
	fetcher := &stubFetcher{bars: []core.Bar{{Date: d1, Close: 100}, {Date: d2, Close: 120}}}
	dir := &stubDir{err: errors.New("offline")}

	k := core.NewKernel()
	k.SetPriceStore(store)
	k.SetDirectoryStore(store)
	_ = k.RegisterMarket(core.MarketSpec{ID: "us", Currency: "USD", PriceProviders: []string{"stub"}, DirectoryProvider: "stub_dir"}, false)
	_ = k.RegisterPriceProvider(fetcher, false)
	_ = k.RegisterDirectoryProvider(dir, false)

	pos := core.Position{Market: "us", Symbol: "VTI", Currency: "USD"}
	bars, err := k.LoadHistory(ctx, pos, d1, d2, false, nil)
	if err != nil || len(bars) != 2 || fetcher.called != 1 {
		t.Fatalf("miss failed: bars=%v err=%v calls=%d", bars, err, fetcher.called)
	}
	bars, err = k.LoadHistory(ctx, pos, d1, d2, false, nil)
	if err != nil || len(bars) != 2 || fetcher.called != 1 {
		t.Fatalf("cache hit failed: bars=%v err=%v calls=%d", bars, err, fetcher.called)
	}
	bars, err = k.LoadHistory(ctx, pos, d2, d2, true, nil)
	if err != nil || len(bars) != 2 || !bars[0].Date.Equal(d1) {
		t.Fatalf("fullCache dropped older bars: %#v err=%v", bars, err)
	}

	if err := store.PutAssets(ctx, "us", []core.Asset{{Symbol: "VTI", Name: "Vanguard", Market: "us", Kind: "etf"}}); err != nil {
		t.Fatal(err)
	}
	res := k.LoadDirectory(ctx, "us", false)
	if dir.called != 0 || len(res.Assets) != 1 || res.NeedsRefresh {
		t.Fatalf("expected fresh cache hit: %#v calls=%d", res, dir.called)
	}
	res = k.LoadDirectory(ctx, "us", true)
	if dir.called != 1 || res.Assets != nil {
		t.Fatalf("expected offline failure without overwriting: %#v", res)
	}
}
