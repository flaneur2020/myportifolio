package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"portifolio/internal/plugin"
	"portifolio/internal/ui/theme"
)

func writePlugin(t *testing.T, configPath, filename, body string) {
	t.Helper()
	dir := filepath.Join(filepath.Dir(configPath), "plugins")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

const demoPlugin = `
return {
  api_version = 1,
  cards = {
    {
      id = "demo",
      title = "Demo Plugin Card",
      supports_interval = true,
      supports_pan = true,
      render = function(ctx)
        return {
          { type = "text", text = "hello from lua" },
          { type = "metrics", rows = { {"Alpha", "1.50"}, {"Beta", "-2.00"} } },
          { type = "table", headers = {"Symbol", "Weight"}, rows = { {"AAA", "60%"}, {"BBB", "40%"} } },
          { type = "line", points = ctx.curve, height = 4 },
          { type = "bars", rows = { {label="AAA", value=5.0}, {label="BBB", value=-3.0} } },
        }
      end,
    },
  },
}
`

func TestLuaPluginLoadsAndRendersDeclarativeBlocks(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	writePlugin(t, configPath, "demo.lua", demoPlugin)
	c, _ := config.New(config.MarketUS)
	m := New(&c, configPath, nil)
	defer m.Close()

	if len(m.analysis.pluginRaw) != 1 || len(m.analysis.pluginErrors) != 0 {
		t.Fatalf("plugin load failed: cards=%v errors=%v", len(m.analysis.pluginRaw), m.analysis.pluginErrors)
	}
	card := m.analysis.cardByID("demo")
	if card == nil || card.Title() != "Demo Plugin Card" || !card.SupportsInterval() || !card.SupportsPan() {
		t.Fatalf("wrong metadata: %#v", card)
	}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	view := card.Render(analysisRenderContext{
		data: domain.AnalysisData{
			Portfolio: c.Portfolio,
			Curve:     []domain.Point{{Date: start, Value: 100}, {Date: start.AddDate(0, 0, 1), Value: 102}, {Date: start.AddDate(0, 0, 2), Value: 99}},
		},
		grain: domain.GrainDay,
		width: 90,
		style: theme.CardStyle(90, false),
	})
	for _, want := range []string{"Demo Plugin Card", "hello from lua", "Alpha", "1.50", "Symbol", "Weight", "AAA", "BBB", "+5.00%"} {
		if !strings.Contains(view, want) {
			t.Fatalf("plugin render missing %q:\n%s", want, view)
		}
	}
}

func TestLuaPluginSandboxRejectsSystemAccess(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	writePlugin(t, configPath, "bad.lua", `local secret = os.getenv("HOME") return {api_version=1, cards={{id="bad",title="Bad",render=function() return secret end}}}`)
	c, _ := config.New(config.MarketUS)
	m := New(&c, configPath, nil)
	defer m.Close()
	if len(m.analysis.pluginRaw) != 0 || len(m.analysis.pluginErrors) == 0 {
		t.Fatalf("sandbox did not reject os access: cards=%v errors=%v", len(m.analysis.pluginRaw), m.analysis.pluginErrors)
	}
}

func TestPluginCardParticipatesInAnalysisNavigationAndLayout(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	writePlugin(t, configPath, "demo.lua", demoPlugin)
	c, _ := config.New(config.MarketUS)
	c.Portfolio.StartDate = "2024-01-01"
	c.Portfolio.Positions = []config.Position{{Symbol: "VTI", Market: config.MarketUS, Kind: config.KindETF, Currency: "USD", Allocation: 1}}
	m := New(&c, configPath, nil)
	defer m.Close()

	m.page = analysisPage
	m.width = 100
	m.height = 80
	m.bars["VTI"] = []domain.Bar{
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Close: 100},
		{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Close: 102},
		{Date: time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC), Close: 99},
	}
	view := m.viewAnalysis(100)
	if !strings.Contains(view, "Demo Plugin Card") || !strings.Contains(view, "hello from lua") {
		t.Fatalf("plugin card missing from analysis page: %s", view)
	}
	if !strings.Contains(m.viewAnalysisViewport(100), "Card 1/8") {
		t.Fatal("viewport status does not include plugin card count")
	}

	idx := m.analysis.visibleIndex(m.cfg.Analysis, "demo")
	m.analysis.card = idx
	m.analysis.active = true
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.analysis.grain != domain.GrainWeek {
		t.Fatal("active plugin card did not receive interval shortcut")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.analysis.offset("demo") != 1 {
		t.Fatal("plugin card did not receive pan offset")
	}

	// Remove, add back, move, and persist.
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.analysis.visibleIndex(m.cfg.Analysis, "demo") >= 0 {
		t.Fatal("plugin card was not removed")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.analysis.visibleIndex(m.cfg.Analysis, "demo") < 0 {
		t.Fatal("plugin card was not added back")
	}
	m.analysis.card = m.analysis.visibleIndex(m.cfg.Analysis, "demo")
	before := m.analysis.card
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	if m.analysis.card != before-1 || m.analysis.cardAt(m.cfg.Analysis, m.analysis.card).ID() != "demo" {
		t.Fatal("plugin card did not move up")
	}
	saved, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(saved.Analysis.Hidden, "demo") || len(saved.Analysis.Order) == 0 {
		t.Fatal("plugin layout was not persisted")
	}
}

func TestExamplePluginRendersThroughAdapter(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "plugins", "drawdown_summary.lua")
	cards, err := plugin.LoadFile(path)
	if err != nil {
		t.Fatalf("example plugin is invalid: %v", err)
	}
	defer plugin.CloseAll(cards)
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	curve := []domain.Point{
		{Date: start, Value: 100},
		{Date: start.AddDate(0, 0, 1), Value: 90},
		{Date: start.AddDate(0, 0, 2), Value: 95},
	}
	view := pluginCardAdapter{card: cards[0]}.Render(analysisRenderContext{
		data:  domain.AnalysisData{Curve: curve},
		grain: domain.GrainDay,
		width: 80,
		style: theme.CardStyle(80, false),
	})
	if !strings.Contains(view, "Drawdown Summary") || !strings.Contains(view, "Worst drawdown") {
		t.Fatalf("example plugin did not render:\n%s", view)
	}
}
