package plugin

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"portifolio/internal/domain"
)

func TestLoadAndEvaluateBlocks(t *testing.T) {
	dir := t.TempDir()
	body := `
return {
  api_version = 1,
  cards = {{
    id = "demo",
    title = "Demo",
    supports_interval = true,
    supports_pan = true,
    render = function(ctx)
      return {
        { type = "text", text = "hello\27[31m world" },
        { type = "metrics", rows = { {"Alpha", "1.50"} } },
        { type = "table", headers = {"Symbol"}, rows = { {"AAA"} } },
        { type = "line", title = "Curve", points = ctx.curve, height = 5 },
        { type = "bars", rows = { {label="AAA", value=5.0} } },
      }
    end,
  }},
}`
	if err := os.WriteFile(filepath.Join(dir, "demo.lua"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	cards, errs := LoadDir(dir, []string{"information"})
	defer CloseAll(cards)
	if len(cards) != 1 || len(errs) != 0 {
		t.Fatalf("unexpected load result: cards=%d errs=%v", len(cards), errs)
	}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	blocks, err := cards[0].Evaluate(Input{
		Curve: []domain.Point{{Date: start, Value: 100}, {Date: start.AddDate(0, 0, 1), Value: 102}},
		Grain: domain.GrainDay,
		Width: 80,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 5 || blocks[0].Text != "hello world" || blocks[1].MetricRows[0][0] != "Alpha" {
		t.Fatalf("unexpected blocks: %#v", blocks)
	}
}

func TestSandboxRejectsOSAndReservedID(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.lua"), []byte(`local x = os.getenv("HOME") return {api_version=1, cards={{id="bad",title="Bad",render=function() return x end}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dup.lua"), []byte(`return {api_version=1, cards={{id="history",title="Dup",render=function() return {type="text",text="x"} end}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	cards, errs := LoadDir(dir, []string{"history"})
	defer CloseAll(cards)
	if len(cards) != 0 || len(errs) != 2 {
		t.Fatalf("expected 0 cards and 2 errors, got cards=%d errs=%v", len(cards), errs)
	}
}

func TestExamplePluginLoadsAndEvaluates(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "plugins", "drawdown_summary.lua")
	cards, err := LoadFile(path)
	if err != nil {
		t.Fatalf("example plugin failed to load: %v", err)
	}
	defer CloseAll(cards)
	if len(cards) != 1 || cards[0].ID != "drawdown-summary" {
		t.Fatalf("unexpected example cards: %#v", cards)
	}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	blocks, err := cards[0].Evaluate(Input{
		Curve: []domain.Point{
			{Date: start, Value: 100},
			{Date: start.AddDate(0, 0, 1), Value: 90},
			{Date: start.AddDate(0, 0, 2), Value: 95},
		},
		Grain: domain.GrainDay,
		Width: 80,
	})
	if err != nil || len(blocks) == 0 {
		t.Fatalf("example plugin evaluation failed: blocks=%#v err=%v", blocks, err)
	}
}
