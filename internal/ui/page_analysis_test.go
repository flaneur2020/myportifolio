package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"portifolio/internal/ui/theme"
)

func layoutModel(t *testing.T) Model {
	t.Helper()
	c, _ := config.New(config.MarketUS)
	return New(&c, filepath.Join(t.TempDir(), "config.toml"), nil)
}

func TestAnalysisCardRemoveConfirmAddMovePersist(t *testing.T) {
	m := layoutModel(t)
	m.page = analysisPage
	builtinCount := len(builtinAnalysisCards())
	m.analysis.card = m.analysis.visibleIndex(m.cfg.Analysis, "drawdown")
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !m.analysis.confirmDelete {
		t.Fatal("x did not ask for confirmation")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.analysis.confirmDelete || m.analysis.visibleIndex(m.cfg.Analysis, "drawdown") < 0 {
		t.Fatal("Esc should cancel removal without hiding the card")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.analysis.visibleIndex(m.cfg.Analysis, "drawdown") >= 0 || len(m.analysis.visibleCards(m.cfg.Analysis)) != builtinCount-1 {
		t.Fatal("confirmed removal did not hide card")
	}
	saved, err := config.Load(m.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(saved.Analysis.Hidden, "drawdown") {
		t.Fatal("hidden card was not persisted")
	}
	// Add it back after the current card.
	m.analysis.card = m.analysis.visibleIndex(m.cfg.Analysis, "allocation")
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !m.analysis.addMode {
		t.Fatal("a did not open add-card list")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.analysis.visibleIndex(m.cfg.Analysis, "drawdown") < 0 {
		t.Fatal("card was not added back")
	}
	// Move the readded card up, then down.
	m.analysis.card = m.analysis.visibleIndex(m.cfg.Analysis, "drawdown")
	before := m.analysis.card
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	if m.analysis.card != before-1 || m.analysis.cardAt(m.cfg.Analysis, m.analysis.card).ID() != "drawdown" {
		t.Fatal("K did not move card up")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if m.analysis.card != before {
		t.Fatal("J did not move card down")
	}
	saved, err = config.Load(m.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Analysis.Order) == 0 {
		t.Fatal("card order was not persisted")
	}
}

func TestLastAnalysisCardCannotBeRemoved(t *testing.T) {
	m := layoutModel(t)
	builtins := builtinAnalysisCards()
	for _, c := range builtins[1:] {
		m.cfg.Analysis.Hidden = append(m.cfg.Analysis.Hidden, c.ID())
	}
	m.analysis.card = 0
	if len(m.analysis.visibleCards(m.cfg.Analysis)) != 1 {
		t.Fatal("setup failed")
	}
	m.page = analysisPage
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.analysis.confirmDelete {
		t.Fatal("last card should not enter removal confirmation")
	}
}

func TestAnalysisRenderingFollowsConfiguredOrder(t *testing.T) {
	m := layoutModel(t)
	m.cfg.Analysis.Order = []string{"allocation", "information", "history", "drawdown", "returns", "drift", "contribution"}
	if m.analysis.cardAt(m.cfg.Analysis, 0).ID() != "allocation" || m.analysis.cardAt(m.cfg.Analysis, 1).ID() != "information" {
		t.Fatalf("unexpected order: %v", m.analysis.orderedIDs(m.cfg.Analysis))
	}
	if title := m.analysis.cardAt(m.cfg.Analysis, 0).Title(); title != "TARGET ALLOCATION" {
		t.Fatalf("wrong title %q", title)
	}
}

func TestTabActivatesNextCardOnlyWhileActive(t *testing.T) {
	c, _ := config.New(config.MarketUS)
	m := New(&c, "", nil)
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.page != analysisPage || m.analysis.active {
		t.Fatal("Tab from My Portfolio opens inactive Analysis")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.page != holdingsPage || m.analysis.active {
		t.Fatal("inactive Tab must return to My Portfolio")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.analysis.active || m.analysis.card != 0 {
		t.Fatal("Enter did not activate the first card")
	}
	for want := 1; want <= 6; want++ {
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
		if m.analysis.card != want || !m.analysis.active {
			t.Fatalf("active Tab should activate card %d, got %d active=%v", want, m.analysis.card, m.analysis.active)
		}
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.analysis.card != 6 || !m.analysis.active {
		t.Fatal("active Tab must clamp at final card")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.analysis.active || m.page != analysisPage {
		t.Fatal("Esc must leave active mode")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.page != holdingsPage {
		t.Fatal("inactive Tab should switch pages again")
	}
}

func TestAnalysisCardActivationAndControls(t *testing.T) {
	c, _ := config.New(config.MarketUS)
	m := New(&c, "", nil)
	m.page = analysisPage
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.analysis.card != 1 || m.analysis.active || m.analysis.grain != domain.GrainDay {
		t.Fatal("Down must select card while inactive")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.analysis.offset("history") != 0 {
		t.Fatal("inactive card must not pan")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.analysis.active || m.analysis.card != 1 {
		t.Fatal("Enter did not activate selected card")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.analysis.offset("history") != 1 {
		t.Fatal("active history card did not pan")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.analysis.grain != domain.GrainWeek || m.analysis.card != 1 {
		t.Fatal("active Up/Down must change interval, not selection")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.analysis.grain != domain.GrainDay {
		t.Fatal("active Up did not select finer interval")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.analysis.active || m.page != analysisPage {
		t.Fatal("Esc must return to card selection without leaving Analysis")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.analysis.card != 2 {
		t.Fatal("card navigation did not resume after Esc")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.analysis.offset("drawdown") != 1 || m.analysis.offset("history") != 1 {
		t.Fatal("active drawdown card panned the wrong chart")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if m.analysis.grain != domain.GrainWeek || m.analysis.card != 2 {
		t.Fatal("z must preserve active card")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.analysis.grain != domain.GrainWeek || m.analysis.card != 3 {
		t.Fatal("informational card must stay selected without changing interval")
	}
}

func TestCtrlArrowAndShortcutIntervals(t *testing.T) {
	c, _ := config.New(config.MarketUS)
	m := New(&c, "", nil)
	m.page = analysisPage
	m.analysis.card = 2
	m.analysis.setOffset("history", 3)
	for _, want := range []domain.Grain{domain.GrainWeek, domain.GrainMonth, domain.GrainQuarter, domain.GrainYear, domain.GrainYear} {
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlDown})
		if m.analysis.grain != want || m.analysis.card != 2 || m.analysis.offset("history") != 3 {
			t.Fatalf("Ctrl+Down: grain=%s card=%d offset=%d", m.analysis.grain, m.analysis.card, m.analysis.offset("history"))
		}
	}
	for _, want := range []domain.Grain{domain.GrainQuarter, domain.GrainMonth, domain.GrainWeek, domain.GrainDay, domain.GrainDay} {
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlUp})
		if m.analysis.grain != want {
			t.Fatalf("expected %s, got %s", want, m.analysis.grain)
		}
	}
	for _, tc := range []struct {
		key   rune
		grain domain.Grain
	}{{'d', domain.GrainDay}, {'w', domain.GrainWeek}, {'m', domain.GrainMonth}, {'c', domain.GrainQuarter}, {'y', domain.GrainYear}} {
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
		if m.analysis.grain != tc.grain {
			t.Fatalf("%c: got %s", tc.key, m.analysis.grain)
		}
	}
	m.page = detailPage
	m.detail.grain = domain.GrainMonth
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlDown})
	if m.detail.grain != domain.GrainQuarter {
		t.Fatal("asset details Ctrl+Down failed")
	}
}

func TestAnalysisCardSelectedAndActiveStylesDiffer(t *testing.T) {
	selected := theme.AnalysisCardStyle(80, true, false)
	inactive := theme.AnalysisCardStyle(80, false, false)
	if fmt.Sprint(selected.GetBorderTopForeground()) == fmt.Sprint(inactive.GetBorderTopForeground()) {
		t.Fatal("selected card must use a distinct highlight colour")
	}
	if strings.Contains(selected.Render("card"), "╔") {
		t.Fatal("inactive-selected card must not use active border")
	}
	active := theme.AnalysisCardStyle(80, true, true)
	if fmt.Sprint(active.GetBorderStyle()) == fmt.Sprint(selected.GetBorderStyle()) {
		t.Fatal("active card must use a distinct border style")
	}
	if !strings.Contains(active.Render("card"), "╔") || !strings.Contains(active.Render("card"), "╗") {
		t.Fatal("active card must render a double border")
	}
}

func TestAnalysisNormalizesWithoutChangingSavedWeights(t *testing.T) {
	c, _ := config.New(config.MarketUS)
	c.Portfolio.StartDate = "2024-01-01"
	c.Portfolio.Positions = []config.Position{{Symbol: "A", Allocation: .2}, {Symbol: "B", Allocation: .3}, {Symbol: "ZERO", Allocation: 0}}
	m := New(&c, "", nil)
	d := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	m.bars["A"] = []domain.Bar{{Date: d, Close: 100}, {Date: d.AddDate(0, 0, 1), Close: 120}}
	m.bars["B"] = []domain.Bar{{Date: d, Close: 100}, {Date: d.AddDate(0, 0, 1), Close: 100}}
	view := m.viewAnalysis(80)
	if !strings.Contains(view, "108.00") || !strings.Contains(view, "normalized to 100%") {
		t.Fatalf("unexpected normalized return: %s", view)
	}
	if m.cfg.Portfolio.Positions[0].Allocation != .2 || m.cfg.Portfolio.Positions[1].Allocation != .3 {
		t.Fatal("saved weights mutated")
	}

	c2, _ := config.New(config.MarketUS)
	c2.Portfolio.Positions = []config.Position{{Symbol: "A"}}
	m2 := New(&c2, "", nil)
	if !strings.Contains(m2.viewAnalysis(80), "positive target weight") {
		t.Fatal("missing zero-weight guidance")
	}
}

func TestPortfolioInformationAndAlignment(t *testing.T) {
	c, _ := config.New(config.MarketUS)
	c.Portfolio.StartDate = "2024-01-01"
	c.Portfolio.InitialCapital = 10000
	c.Portfolio.Positions = []config.Position{{Symbol: "VTI", Allocation: 1}}
	m := New(&c, "", nil)
	m.page = analysisPage
	m.bars["VTI"] = []domain.Bar{
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Close: 100},
		{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Close: 110},
		{Date: time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC), Close: 99},
	}
	view := m.viewAnalysis(120)
	for _, want := range []string{"Recent day", "2024-01-04", "-10.00%", "-1100.00 USD"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}

	plain := ansi.Strip(view)
	text := plain[strings.Index(plain, "PORTFOLIO INFORMATION"):]
	labels := []string{"Model index", "Total return", "Max drawdown", "Recent day", "Requested start", "Effective start", "Latest data", "Rebalance", "Initial capital", "Model value"}
	column := -1
	for _, label := range labels {
		line := label + strings.Repeat(" ", 17-len(label)) + "  "
		pos := strings.Index(text, line)
		if pos < 0 {
			t.Fatalf("missing aligned row %q", label)
		}
		lineStart := strings.LastIndex(text[:pos], "\n") + 1
		valueColumn := pos - lineStart + len(line)
		if column < 0 {
			column = valueColumn
		} else if valueColumn != column {
			t.Fatalf("%s value at %d, expected %d", label, valueColumn, column)
		}
	}
}

func TestAnalysisCardsScrollingAndBuiltinRenders(t *testing.T) {
	c, _ := config.New(config.MarketUS)
	c.Portfolio.StartDate = "2024-01-01"
	c.Portfolio.Positions = []config.Position{{Symbol: "AAA", Allocation: .5}, {Symbol: "BBB", Allocation: .5}}
	m := New(&c, "", nil)
	m.page = analysisPage
	m.height = 24
	m.width = 100
	m.bars["AAA"] = []domain.Bar{
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Close: 100},
		{Date: time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC), Close: 110},
	}
	m.bars["BBB"] = []domain.Bar{
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Close: 100},
		{Date: time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC), Close: 90},
	}
	full := m.viewAnalysis(100)
	for _, heading := range []string{"PORTFOLIO INFORMATION", "PORTFOLIO HISTORY", "DRAWDOWN / HISTORY", "TARGET ALLOCATION", "PERIOD RETURNS", "WEIGHT DRIFT", "ASSET CONTRIBUTION"} {
		if !strings.Contains(full, heading) {
			t.Fatalf("missing chart %q", heading)
		}
	}
	for _, want := range []string{"LATEST ALLOCATION", "DRIFT BY ASSET", "+5.0pp", "-5.0pp", "Total", "+500.00 USD", "-500.00 USD"} {
		if !strings.Contains(full, want) {
			t.Fatalf("missing detail %q", want)
		}
	}
	before := m.View()
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyPgDown})
	if m.analysis.scroll == 0 || m.View() == before {
		t.Fatal("page cannot scroll")
	}
	if m.analysis.grain != domain.GrainDay || m.analysis.offset("history") != 0 {
		t.Fatal("scroll changed chart controls")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyPgUp})
	if m.analysis.scroll != 0 {
		t.Fatal("cannot return to top")
	}
}

func TestHistoryCardOffsetAndYearlyCoverage(t *testing.T) {
	var curve []domain.Point
	for i := 0; i < 100; i++ {
		curve = append(curve, domain.Point{Date: time.Date(2025, 1, 1+i, 0, 0, 0, 0, time.UTC), Value: 100 + float64(i)})
	}
	view := renderHistoryCard(analysisRenderContext{
		data:   domain.AnalysisData{Curve: curve},
		grain:  domain.GrainDay,
		offset: 1,
		width:  80,
		style:  theme.CardStyle(80, false),
	})
	want := curve[98].Date.Format("2006-01-02")
	if !strings.Contains(view, "→ "+want) {
		t.Fatalf("one step should end at %s: %s", want, view)
	}
	if strings.Contains(view, "•") || !strings.Contains(view, "█") {
		t.Fatal("analysis does not use candlesticks")
	}

	yearly := []domain.Point{
		{Date: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), Value: 100},
		{Date: time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC), Value: 120},
		{Date: time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC), Value: 125},
		{Date: time.Date(2021, 12, 31, 0, 0, 0, 0, time.UTC), Value: 130},
	}
	view = renderHistoryCard(analysisRenderContext{
		data:  domain.AnalysisData{Curve: yearly},
		grain: domain.GrainYear,
		width: 80,
		style: theme.CardStyle(80, false),
	})
	if !strings.Contains(view, "Window: 2020-01-02 → 2021-12-31") {
		t.Fatalf("incorrect yearly coverage: %s", view)
	}
}

func TestPeriodReturnsCardAndDriftContributionPaging(t *testing.T) {
	curve := []domain.Point{
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Value: 100},
		{Date: time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), Value: 120},
		{Date: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), Value: 90},
		{Date: time.Date(2024, 3, 29, 0, 0, 0, 0, time.UTC), Value: 99},
	}
	c, _ := config.New(config.MarketUS)
	view := renderPeriodReturnsCard(analysisRenderContext{
		data:  domain.AnalysisData{Portfolio: c.Portfolio, Curve: curve, Capital: 10000},
		grain: domain.GrainMonth,
		width: 100,
		style: theme.CardStyle(100, false),
	})
	for _, want := range []string{"PERIOD RETURNS · MONTH", "$0", "Latest  2024-03-29", "+10.00%", "+900.00 USD", "━━/░░ Gain above 0%", "━━/░░ Loss below 0%"} {
		if !strings.Contains(view, want) {
			t.Fatalf("bar chart missing %q:\n%s", want, view)
		}
	}
	if strings.Index(view, "Latest") > strings.Index(view, "$0") {
		t.Fatal("latest revenue summary should appear above the chart")
	}

	c.Portfolio.Positions = []config.Position{{Symbol: "AAA", Allocation: 1}}
	m := New(&c, "", nil)
	m.page = analysisPage
	m.analysis.card = 5
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if m.analysis.grain != domain.GrainWeek {
		t.Fatal("z did not change interval for drift card")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.analysis.offset("drift") != 1 {
		t.Fatal("drift history did not pan")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.analysis.offset("contribution") != 1 {
		t.Fatal("contribution periods did not pan")
	}
}
