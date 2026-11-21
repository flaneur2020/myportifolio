package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"portifolio/internal/config"
	"portifolio/internal/domain"
)

func TestSQLitePriceRoundTrip(t *testing.T) {
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := config.Position{Market: config.MarketUS, Symbol: "SPY"}
	d := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	if err = s.Put(context.Background(), p, []domain.Bar{{Date: d, Close: 470, AdjustedClose: 468, Currency: "USD", Source: "mock"}}); err != nil {
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
