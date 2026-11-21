package domain

import (
	"math"
	"portifolio/internal/config"
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
func TestSnapshotsExposeWeightsAndRebalance(t *testing.T) {
	a := AssetSeries{Position: config.Position{Allocation: .5}, Points: []Point{{day(2024, 1, 2), 100}, {day(2024, 2, 1), 100}, {day(2024, 2, 2), 100}, {day(2024, 3, 1), 100}}}
	b := AssetSeries{Position: config.Position{Allocation: .5}, Points: []Point{{day(2024, 1, 2), 100}, {day(2024, 2, 1), 200}, {day(2024, 2, 2), 200}, {day(2024, 3, 1), 200}}}
	never := BuildPortfolioSnapshots(config.Portfolio{StartDate: "2024-01-01", Rebalance: "never"}, []AssetSeries{a, b})
	if len(never) != 4 {
		t.Fatal("missing snapshots")
	}
	if math.Abs(never[2].Weights[0]-1.0/3) > 1e-9 || math.Abs(never[2].Weights[1]-2.0/3) > 1e-9 {
		t.Fatalf("wrong drift weights: %#v", never[2].Weights)
	}
	monthly := BuildPortfolioSnapshots(config.Portfolio{StartDate: "2024-01-01", Rebalance: "monthly"}, []AssetSeries{a, b})
	last := monthly[len(monthly)-1]
	if math.Abs(last.Weights[0]-.5) > 1e-9 || math.Abs(last.Weights[1]-.5) > 1e-9 {
		t.Fatalf("rebalance did not restore weights: %#v", last.Weights)
	}
}
