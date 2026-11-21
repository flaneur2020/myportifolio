package ui

import (
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestEffectiveStartExplainsLateAsset(t *testing.T) {
	early := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	late := time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC)
	series := []domain.AssetSeries{
		{Position: config.Position{Symbol: "SPY", Allocation: .5}, Points: []domain.Point{{Date: early, Value: 100}, {Date: late, Value: 110}}},
		{Position: config.Position{Symbol: "SGOV", Allocation: .5}, Points: []domain.Point{{Date: late, Value: 100}}},
	}
	curve := domain.BuildPortfolio(config.Portfolio{StartDate: "2020-01-01", Rebalance: "never"}, series)
	if len(curve) != 1 || !curve[0].Date.Equal(late) {
		t.Fatal("model should begin at common price date")
	}
	note := effectiveStartNote("2020-01-01", curve[0].Date, series)
	if !strings.Contains(note, "SGOV history begins 2020-06-01") || !strings.Contains(note, "Effective start 2020-06-01") {
		t.Fatalf("missing explanation: %s", note)
	}
}
