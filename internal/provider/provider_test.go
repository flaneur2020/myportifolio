package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"portifolio/internal/config"
	"portifolio/internal/domain"
)

type fakeProvider struct {
	name   string
	fail   bool
	called *int
}

func (f fakeProvider) Name() string      { return f.name }
func (f fakeProvider) Markets() []string { return []string{config.MarketUS} }
func (f fakeProvider) History(context.Context, config.Position, time.Time, time.Time) ([]domain.Bar, error) {
	*f.called++
	if f.fail {
		return nil, errors.New("failed")
	}
	return []domain.Bar{{Close: 1}}, nil
}

func TestRegistryFallbackOrder(t *testing.T) {
	first, second := 0, 0
	r := NewRegistry()
	r.Register(fakeProvider{"first", true, &first})
	r.Register(fakeProvider{"second", false, &second})
	markets := map[string]config.Market{config.MarketUS: {Providers: []config.ProviderConfig{{Name: "first", Enabled: true}, {Name: "second", Enabled: true}}}}
	_, err := r.History(context.Background(), markets, config.Position{Market: config.MarketUS}, time.Time{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if first != 1 || second != 1 {
		t.Fatalf("unexpected calls first=%d second=%d", first, second)
	}
}
