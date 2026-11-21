package ui

import (
	"slices"
	"strings"
	"sync"

	"portifolio/core"
	"portifolio/ui/theme"
)

type evalCacheKey struct {
	cardID       string
	grain        core.Grain
	offset       int
	width        int
	name         string
	startDate    string
	rebalance    string
	capital      float64
	configWeight float64
	curveLen     int
	firstDate    int64
	firstVal     float64
	lastDate     int64
	lastVal      float64
	snapLen      int
	barCount     int
}

type cachedEval struct {
	res core.CardResult
	err error
}

type cardEvalCache struct {
	mu    sync.RWMutex
	items map[evalCacheKey]cachedEval
}

func newCardEvalCache() *cardEvalCache {
	return &cardEvalCache{items: map[evalCacheKey]cachedEval{}}
}

func (c *cardEvalCache) evaluate(card core.Card, in core.CardInput) (core.CardResult, error) {
	if c == nil {
		return card.Evaluate(in)
	}
	key := makeEvalKey(card.Meta().ID, in)
	c.mu.RLock()
	if hit, ok := c.items[key]; ok {
		c.mu.RUnlock()
		return hit.res, hit.err
	}
	c.mu.RUnlock()

	res, err := card.Evaluate(in)
	c.mu.Lock()
	if len(c.items) > 128 {
		clear(c.items)
	}
	c.items[key] = cachedEval{res: res, err: err}
	c.mu.Unlock()
	return res, err
}

func makeEvalKey(id string, in core.CardInput) evalCacheKey {
	d := in.Analysis
	key := evalCacheKey{
		cardID:       id,
		grain:        in.Grain,
		offset:       in.Offset,
		width:        in.Width,
		name:         d.Portfolio.Name,
		startDate:    d.Portfolio.StartDate,
		rebalance:    d.Portfolio.Rebalance,
		capital:      d.Capital,
		configWeight: d.ConfiguredWeight,
		curveLen:     len(d.Curve),
		snapLen:      len(d.Snapshots),
	}
	if len(d.Curve) > 0 {
		key.firstDate = d.Curve[0].Date.Unix()
		key.firstVal = d.Curve[0].Value
		last := d.Curve[len(d.Curve)-1]
		key.lastDate = last.Date.Unix()
		key.lastVal = last.Value
	}
	for _, b := range in.Bars {
		key.barCount += len(b)
	}
	return key
}

type analysisState struct {
	grain         core.Grain
	card          int
	scroll        int
	active        bool
	confirmDelete bool
	addMode       bool
	addCursor     int
	offsets       map[string]int
	cards         []core.Card
	pluginErrors  []string
	evalCache     *cardEvalCache
}

func newAnalysisState(kernel *core.Kernel) analysisState {
	var cards []core.Card
	var pluginErrs []string
	if kernel != nil {
		cards = kernel.Cards()
		pluginErrs = kernel.PluginErrors()
	}
	return analysisState{
		grain:        core.GrainDay,
		offsets:      map[string]int{},
		cards:        cards,
		pluginErrors: pluginErrs,
		evalCache:    newCardEvalCache(),
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

func (a analysisState) cardByID(id string) core.Card {
	for _, c := range a.cards {
		if c.Meta().ID == id {
			return c
		}
	}
	return nil
}

func (a analysisState) orderedIDs(cfg core.AnalysisConfig) []string {
	known := make(map[string]bool, len(a.cards))
	for _, c := range a.cards {
		known[c.Meta().ID] = true
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
		id := c.Meta().ID
		if !seen[id] {
			out = append(out, id)
		}
	}
	return out
}

func (a analysisState) visibleCards(cfg core.AnalysisConfig) []core.Card {
	hidden := make(map[string]bool, len(cfg.Hidden))
	for _, id := range cfg.Hidden {
		hidden[id] = true
	}
	var out []core.Card
	for _, id := range a.orderedIDs(cfg) {
		if !hidden[id] {
			if c := a.cardByID(id); c != nil {
				out = append(out, c)
			}
		}
	}
	return out
}

func (a analysisState) hiddenCards(cfg core.AnalysisConfig) []core.Card {
	hidden := make(map[string]bool, len(cfg.Hidden))
	for _, id := range cfg.Hidden {
		hidden[id] = true
	}
	var out []core.Card
	for _, id := range a.orderedIDs(cfg) {
		if hidden[id] {
			if c := a.cardByID(id); c != nil {
				out = append(out, c)
			}
		}
	}
	return out
}

func (a analysisState) visibleIndex(cfg core.AnalysisConfig, id string) int {
	for i, c := range a.visibleCards(cfg) {
		if c.Meta().ID == id {
			return i
		}
	}
	return -1
}

func (a analysisState) cardAt(cfg core.AnalysisConfig, index int) core.Card {
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
	if err := m.kernel.SaveConfig(m.cfgPath, c); err != nil {
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
	meta := card.Meta()
	hidden := append([]string(nil), m.cfg.Analysis.Hidden...)
	if !slices.Contains(hidden, meta.ID) {
		hidden = append(hidden, meta.ID)
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
	m.status = "Removed card: " + meta.Title + " · press a to add it back"
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
	target := hidden[index].Meta()
	order := removeString(m.analysis.orderedIDs(m.cfg.Analysis), target.ID)
	current := m.analysis.cardAt(m.cfg.Analysis, m.analysis.card)
	inserted := false
	if current != nil {
		currentID := current.Meta().ID
		for i, existing := range order {
			if existing == currentID {
				order = slices.Insert(order, i+1, target.ID)
				inserted = true
				break
			}
		}
	}
	if !inserted {
		order = append(order, target.ID)
	}
	hiddenList := removeString(append([]string(nil), m.cfg.Analysis.Hidden...), target.ID)
	if !m.saveAnalysisLayout(order, hiddenList) {
		return false
	}
	m.analysis.addMode = false
	m.analysis.active = false
	m.analysis.card = max(0, m.analysis.visibleIndex(m.cfg.Analysis, target.ID))
	m.status = "Added card: " + target.Title
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
	left, right := visible[from].Meta().ID, visible[to].Meta().ID
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
	m.status = "Moved card: " + visible[from].Meta().Title
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
