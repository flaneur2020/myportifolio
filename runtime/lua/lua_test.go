package luart

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"portifolio/core"
)

func TestLoadAndEvaluateBlocksAndLazyBars(t *testing.T) {
	k := core.NewKernel()
	h := NewHost(HostOptions{})
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
      local vti = ctx.bars["VTI"]
      return {
        { type = "text", text = "hello\27[31m world " .. tostring(#vti) },
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
	errs := h.LoadDir(k, dir)
	if len(errs) != 0 || len(k.Cards()) != 1 {
		t.Fatalf("unexpected load result: cards=%d errs=%v", len(k.Cards()), errs)
	}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	res, err := k.Cards()[0].Evaluate(core.CardInput{
		Analysis: core.AnalysisData{
			Curve: []core.Point{{Date: start, Value: 100}, {Date: start.AddDate(0, 0, 1), Value: 102}},
		},
		Bars: map[string][]core.Bar{
			"VTI": {{Date: start, Close: 100}, {Date: start.AddDate(0, 0, 1), Close: 102}},
		},
		Grain: core.GrainDay,
		Width: 80,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Blocks) != 5 || res.Blocks[0].Text != "hello world 2" || res.Blocks[1].MetricRows[0].Label != "Alpha" {
		t.Fatalf("unexpected blocks: %#v", res.Blocks)
	}
}

func TestSandboxRejectsOSAndDuplicateWithoutOverride(t *testing.T) {
	k := core.NewKernel()
	h := NewHost(HostOptions{})
	// Pre-register a card "history"
	_ = h.LoadBytes(k, "base.lua", []byte(`return {api_version=1, cards={{id="history",title="History",render=function() return {type="text",text="base"} end}}}`))

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.lua"), []byte(`local x = os.getenv("HOME") return {api_version=1, cards={{id="bad",title="Bad",render=function() return x end}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dup.lua"), []byte(`return {api_version=1, cards={{id="history",title="Dup",render=function() return {type="text",text="x"} end}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	errs := h.LoadDir(k, dir)
	if len(errs) != 2 || len(k.Cards()) != 1 {
		t.Fatalf("expected 1 card and 2 errors, got cards=%d errs=%v", len(k.Cards()), errs)
	}

	// Explicit override = true replaces cleanly.
	errs = h.LoadBytes(k, "override.lua", []byte(`return {api_version=1, cards={{id="history",title="Custom History",override=true,render=function() return {type="text",text="overridden"} end}}}`))
	if len(errs) != 0 {
		t.Fatalf("override failed: %v", errs)
	}
	c, _ := k.Card("history")
	if c.Meta().Title != "Custom History" {
		t.Fatalf("card not overridden: %q", c.Meta().Title)
	}
}

func TestCapabilityGatingCardsNeverGetHTTPAndProvidersEnforceAllowedHosts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"date":"2024-01-02","close":123.45}]`)
	}))
	defer srv.Close()

	k := core.NewKernel()
	h := NewHost(HostOptions{
		HTTPClient:  srv.Client(),
		URLRewrites: map[string]string{"https://allowed.example": srv.URL},
	})
	script := `
return {
  api_version = 1,
  capabilities = { "http" },
  allowed_hosts = { "allowed.example" },
  markets = {{ id = "test", name = "Test", currency = "USD", price_providers = { "p1", "p2" } }},
  providers = {
    {
      name = "p1",
      markets = { "test" },
      history = function(req)
        local resp, err = host.http.get("https://allowed.example/prices")
        if err then return nil, err end
        local data = host.json.decode(resp.body)
        return data, nil
      end,
    },
    {
      name = "p2",
      markets = { "test" },
      history = function(req)
        local resp, err = host.http.get("https://evil.example/steal")
        if err then return nil, err end
        return {}, nil
      end,
    },
  },
  cards = {{
    id = "sneaky",
    title = "Sneaky",
    render = function(ctx)
      if host.http ~= nil then
        return { type = "text", text = "LEAKED" }
      end
      return { type = "text", text = "SAFE" }
    end,
  }},
}`
	if errs := h.LoadBytes(k, "cap.lua", []byte(script)); len(errs) != 0 {
		t.Fatalf("load failed: %v", errs)
	}

	// Card must NOT see host.http even though plugin declared capability "http".
	card, _ := k.Card("sneaky")
	res, err := card.Evaluate(core.CardInput{})
	if err != nil || res.Blocks[0].Text != "SAFE" {
		t.Fatalf("card sandbox violated: res=%#v err=%v", res, err)
	}

	// Allowed provider succeeds concurrently (race detector verified).
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bars, err := k.FetchHistory(context.Background(), core.Position{Market: "test", Symbol: "AAA"}, time.Time{}, time.Now())
			if err != nil || len(bars) != 1 || bars[0].Close != 123.45 {
				t.Errorf("concurrent provider fetch failed: bars=%v err=%v", bars, err)
			}
		}()
	}
	wg.Wait()

	// Disallowed host must be blocked.
	_ = k.RegisterMarket(core.MarketSpec{ID: "evil", Currency: "USD", PriceProviders: []string{"p2"}}, false)
	if _, err := k.FetchHistory(context.Background(), core.Position{Market: "evil", Symbol: "AAA"}, time.Time{}, time.Now()); err == nil {
		t.Fatal("disallowed host was not blocked")
	}
}

func TestExamplePluginLoadsAndEvaluates(t *testing.T) {
	k := core.NewKernel()
	h := NewHost(HostOptions{})
	path := filepath.Join("..", "..", "examples", "plugins", "drawdown_summary.lua")
	errs := h.LoadFile(k, path)
	if len(errs) != 0 {
		t.Fatalf("example plugin failed to load: %v", errs)
	}
	card, ok := k.Card("drawdown-summary")
	if !ok {
		t.Fatal("example card not registered")
	}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	res, err := card.Evaluate(core.CardInput{
		Analysis: core.AnalysisData{
			Curve: []core.Point{
				{Date: start, Value: 100},
				{Date: start.AddDate(0, 0, 1), Value: 90},
				{Date: start.AddDate(0, 0, 2), Value: 95},
			},
		},
		Grain: core.GrainDay,
		Width: 80,
	})
	if err != nil || len(res.Blocks) == 0 {
		t.Fatalf("example plugin evaluation failed: res=%#v err=%v", res, err)
	}
}
