package ui

import (
	"portifolio/internal/config"
	"portifolio/internal/provider"
	"strings"
	"testing"
	"time"
)

func TestAnalysisNormalizesWithoutChangingSavedWeights(t *testing.T) {
	c, _ := config.New("us")
	c.Portfolios[0].StartDate = "2024-01-01"
	c.Portfolios[0].Positions = []config.Position{{Symbol: "A", Allocation: .2}, {Symbol: "B", Allocation: .3}, {Symbol: "ZERO", Allocation: 0}}
	m := New(&c, "", nil, nil)
	d := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	m.bars["A"] = []provider.Bar{{Date: d, Close: 100}, {Date: d.AddDate(0, 0, 1), Close: 120}}
	m.bars["B"] = []provider.Bar{{Date: d, Close: 100}, {Date: d.AddDate(0, 0, 1), Close: 100}}
	view := m.analysisView(80)
	if !strings.Contains(view, "108.00") || !strings.Contains(view, "normalized to 100%") {
		t.Fatalf("unexpected normalized return: %s", view)
	}
	if m.cfg.Portfolios[0].Positions[0].Allocation != .2 || m.cfg.Portfolios[0].Positions[1].Allocation != .3 {
		t.Fatal("saved weights mutated")
	}
}
func TestAllZeroWeightsHaveClearMessage(t *testing.T) {
	c, _ := config.New("us")
	c.Portfolios[0].Positions = []config.Position{{Symbol: "A"}}
	m := New(&c, "", nil, nil)
	if !strings.Contains(m.analysisView(80), "positive target weight") {
		t.Fatal("missing zero-weight guidance")
	}
}
