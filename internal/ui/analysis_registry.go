package ui

import (
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"portifolio/internal/plugin"
	"portifolio/internal/ui/theme"
)

type analysisRenderContext struct {
	data   domain.AnalysisData
	bars   map[string][]domain.Bar
	grain  domain.Grain
	offset int
	width  int
	style  lipgloss.Style
}

// analysisCard is the unified contract implemented by both built-in cards and Lua plugins.
type analysisCard interface {
	ID() string
	Title() string
	SupportsInterval() bool
	SupportsPan() bool
	Render(ctx analysisRenderContext) string
}

type analysisState struct {
	grain         domain.Grain
	card          int
	scroll        int
	active        bool
	confirmDelete bool
	addMode       bool
	addCursor     int
	offsets       map[string]int
	cards         []analysisCard
	pluginRaw     []*plugin.Card
	pluginErrors  []string
}

func newAnalysisState(cfgPath string) analysisState {
	builtins := builtinAnalysisCards()
	reserved := make([]string, 0, len(builtins))
	for _, c := range builtins {
		reserved = append(reserved, c.ID())
	}
	plugins, pluginErrs := plugin.LoadForConfig(cfgPath, reserved)
	cards := append([]analysisCard(nil), builtins...)
	for _, p := range plugins {
		cards = append(cards, pluginCardAdapter{card: p})
	}
	return analysisState{
		grain:        domain.GrainDay,
		offsets:      map[string]int{},
		cards:        cards,
		pluginRaw:    plugins,
		pluginErrors: pluginErrs,
	}
}

func (a *analysisState) offset(id string) int {
	if a.offsets == nil {
		return 0
	}
	return a.offsets[id]
}

func (a *analysisState) setOffset(id string, v int) {
	if a.offsets == nil {
		a.offsets = map[string]int{}
	}
	a.offsets[id] = max(0, v)
}

func (a *analysisState) shiftOffset(id string, delta int) {
	a.setOffset(id, a.offset(id)+delta)
}

func (a analysisState) cardByID(id string) analysisCard {
	for _, c := range a.cards {
		if c.ID() == id {
			return c
		}
	}
	return nil
}

func (a analysisState) orderedIDs(cfg config.AnalysisConfig) []string {
	known := make(map[string]bool, len(a.cards))
	for _, c := range a.cards {
		known[c.ID()] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, id := range cfg.Order {
		if known[id] && !seen[id] {
			out = append(out, id)
			seen[id] = true
		}
	}
	for _, c := range a.cards {
		if !seen[c.ID()] {
			out = append(out, c.ID())
		}
	}
	return out
}

func (a analysisState) visibleCards(cfg config.AnalysisConfig) []analysisCard {
	hidden := make(map[string]bool, len(cfg.Hidden))
	for _, id := range cfg.Hidden {
		hidden[id] = true
	}
	var out []analysisCard
	for _, id := range a.orderedIDs(cfg) {
		if !hidden[id] {
			if c := a.cardByID(id); c != nil {
				out = append(out, c)
			}
		}
	}
	return out
}

func (a analysisState) hiddenCards(cfg config.AnalysisConfig) []analysisCard {
	hidden := make(map[string]bool, len(cfg.Hidden))
	for _, id := range cfg.Hidden {
		hidden[id] = true
	}
	var out []analysisCard
	for _, id := range a.orderedIDs(cfg) {
		if hidden[id] {
			if c := a.cardByID(id); c != nil {
				out = append(out, c)
			}
		}
	}
	return out
}

func (a analysisState) visibleIndex(cfg config.AnalysisConfig, id string) int {
	for i, c := range a.visibleCards(cfg) {
		if c.ID() == id {
			return i
		}
	}
	return -1
}

func (a analysisState) cardAt(cfg config.AnalysisConfig, index int) analysisCard {
	visible := a.visibleCards(cfg)
	if index >= 0 && index < len(visible) {
		return visible[index]
	}
	return nil
}

func (a analysisState) pluginErrorsView() string {
	if len(a.pluginErrors) == 0 {
		return ""
	}
	return "\n" + theme.Muted.Render("Plugin load errors: "+strings.Join(a.pluginErrors, "; "))
}

func (m *Model) saveAnalysisLayout(order, hidden []string) bool {
	c := m.cfg.Clone()
	c.Analysis.Order = append([]string(nil), order...)
	c.Analysis.Hidden = append([]string(nil), hidden...)
	if err := config.Save(m.cfgPath, c); err != nil {
		m.status = "Could not save Analysis layout: " + err.Error()
		return false
	}
	m.cfg = c
	m.analysis.scroll = 0
	return true
}

func (m *Model) removeAnalysisCardAtCursor() bool {
	visible := m.analysis.visibleCards(m.cfg.Analysis)
	if len(visible) <= 1 {
		m.status = "At least one Analysis card must remain."
		return false
	}
	card := m.analysis.cardAt(m.cfg.Analysis, m.analysis.card)
	if card == nil {
		return false
	}
	hidden := append([]string(nil), m.cfg.Analysis.Hidden...)
	if !slices.Contains(hidden, card.ID()) {
		hidden = append(hidden, card.ID())
	}
	if !m.saveAnalysisLayout(m.analysis.orderedIDs(m.cfg.Analysis), hidden) {
		return false
	}
	m.analysis.active = false
	m.analysis.confirmDelete = false
	count := len(m.analysis.visibleCards(m.cfg.Analysis))
	if m.analysis.card >= count {
		m.analysis.card = max(0, count-1)
	}
	m.status = "Removed card: " + card.Title() + " · press a to add it back"
	return true
}

func (m *Model) beginAddAnalysisCard() {
	if len(m.analysis.hiddenCards(m.cfg.Analysis)) == 0 {
		m.status = "No hidden cards to add."
		return
	}
	m.analysis.addMode = true
	m.analysis.addCursor = 0
	m.analysis.active = false
	m.status = ""
}

func (m *Model) addAnalysisCardAtIndex(index int) bool {
	hidden := m.analysis.hiddenCards(m.cfg.Analysis)
	if len(hidden) == 0 {
		m.status = "No hidden cards to add."
		return false
	}
	index = max(0, min(index, len(hidden)-1))
	target := hidden[index]
	order := removeString(m.analysis.orderedIDs(m.cfg.Analysis), target.ID())
	current := m.analysis.cardAt(m.cfg.Analysis, m.analysis.card)
	inserted := false
	if current != nil {
		for i, existing := range order {
			if existing == current.ID() {
				order = slices.Insert(order, i+1, target.ID())
				inserted = true
				break
			}
		}
	}
	if !inserted {
		order = append(order, target.ID())
	}
	hiddenList := removeString(append([]string(nil), m.cfg.Analysis.Hidden...), target.ID())
	if !m.saveAnalysisLayout(order, hiddenList) {
		return false
	}
	m.analysis.addMode = false
	m.analysis.active = false
	m.analysis.card = max(0, m.analysis.visibleIndex(m.cfg.Analysis, target.ID()))
	m.status = "Added card: " + target.Title()
	return true
}

func (m *Model) moveAnalysisCard(delta int) bool {
	visible := m.analysis.visibleCards(m.cfg.Analysis)
	from := m.analysis.card
	if from < 0 || from >= len(visible) {
		return false
	}
	to := from + delta
	if to < 0 || to >= len(visible) {
		if delta < 0 {
			m.status = "Card is already first."
		} else {
			m.status = "Card is already last."
		}
		return false
	}
	order := m.analysis.orderedIDs(m.cfg.Analysis)
	left, right := visible[from].ID(), visible[to].ID()
	leftIndex := slices.Index(order, left)
	rightIndex := slices.Index(order, right)
	if leftIndex < 0 || rightIndex < 0 {
		return false
	}
	order[leftIndex], order[rightIndex] = order[rightIndex], order[leftIndex]
	if !m.saveAnalysisLayout(order, m.cfg.Analysis.Hidden) {
		return false
	}
	m.analysis.card = to
	m.analysis.scroll = 0
	m.status = "Moved card: " + visible[from].Title()
	return true
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}
