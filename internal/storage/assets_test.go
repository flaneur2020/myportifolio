package storage

import (
	"context"
	"path/filepath"
	"testing"

	"portifolio/internal/config"
	"portifolio/internal/domain"
)

func TestDirectorySnapshotAtomicAndMarketIsolated(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	us := []domain.Asset{{Symbol: "VTI", Name: "Vanguard", Market: config.MarketUS, Kind: config.KindETF}}
	if err = s.PutAssets(ctx, config.MarketUS, us); err != nil {
		t.Fatal(err)
	}
	cn := []domain.Asset{{Symbol: "510300", Name: "CSI 300", Market: config.MarketCN, Kind: config.KindETF}}
	if err = s.PutAssets(ctx, config.MarketCN, cn); err != nil {
		t.Fatal(err)
	}
	if err = s.PutAssets(ctx, config.MarketUS, nil); err == nil {
		t.Fatal("empty snapshot accepted")
	}
	duplicate := append(append([]domain.Asset{}, us...), us...)
	if err = s.PutAssets(ctx, config.MarketUS, duplicate); err == nil {
		t.Fatal("duplicate snapshot accepted")
	}
	got, stamp, err := s.Assets(ctx, config.MarketUS)
	if err != nil || len(got) != 1 || got[0].Symbol != "VTI" || stamp.IsZero() {
		t.Fatalf("snapshot lost: %#v %v", got, err)
	}
	got, _, err = s.Assets(ctx, config.MarketCN)
	if err != nil || len(got) != 1 {
		t.Fatal("other market lost")
	}
}
