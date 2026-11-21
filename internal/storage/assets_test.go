package storage

import (
	"context"
	"path/filepath"
	"portifolio/internal/provider"
	"testing"
)

func TestDirectorySnapshotAtomicAndMarketIsolated(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	us := []provider.Asset{{Symbol: "VTI", Name: "Vanguard", Market: "us", Kind: "etf"}}
	if err = s.PutAssets(ctx, "us", us); err != nil {
		t.Fatal(err)
	}
	cn := []provider.Asset{{Symbol: "510300", Name: "CSI 300", Market: "cn", Kind: "etf"}}
	if err = s.PutAssets(ctx, "cn", cn); err != nil {
		t.Fatal(err)
	}
	if err = s.PutAssets(ctx, "us", nil); err == nil {
		t.Fatal("empty snapshot accepted")
	}
	duplicate := append(append([]provider.Asset{}, us...), us...)
	if err = s.PutAssets(ctx, "us", duplicate); err == nil {
		t.Fatal("duplicate snapshot accepted")
	}
	got, stamp, err := s.Assets(ctx, "us")
	if err != nil || len(got) != 1 || got[0].Symbol != "VTI" || stamp.IsZero() {
		t.Fatalf("snapshot lost: %#v %v", got, err)
	}
	got, _, err = s.Assets(ctx, "cn")
	if err != nil || len(got) != 1 {
		t.Fatal("other market lost")
	}
}
