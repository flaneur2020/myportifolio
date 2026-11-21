package ui

import (
	"sort"
	"strings"

	"portifolio/core"
)

func (m Model) popularRank(asset core.Asset) int {
	if m.kernel == nil {
		return 10000
	}
	market, ok := m.kernel.Market(asset.Market)
	if !ok {
		return 10000
	}
	for i, symbol := range market.CommonSymbols {
		if asset.Symbol == symbol {
			return i
		}
	}
	return 10000
}

// Match ordered subsequences; prefer exact symbols, prefixes, and compact matches.
func fuzzyScore(query, text string) (int, bool) {
	if text == query || strings.Split(text, " · ")[0] == query {
		return -1000, true
	}
	if strings.HasPrefix(text, query) {
		return -500, true
	}
	if i := strings.Index(text, query); i >= 0 {
		return i, true
	}
	q := []rune(query)
	at, first, last := 0, -1, 0
	for i, r := range []rune(text) {
		if at < len(q) && r == q[at] {
			if first < 0 {
				first = i
			}
			last = i
			at++
		}
	}
	if at != len(q) {
		return 0, false
	}
	return 100 + first + (last - first - len(q)), true
}

func (m Model) filterAssets(query string, holdingsOnly, rankPopular bool) []core.Asset {
	p := m.cfg.Portfolio
	if p.Market == "" {
		return nil
	}
	var all []core.Asset
	for _, s := range m.assets {
		if s.Market == p.Market {
			all = append(all, s)
		}
	}
	for _, pos := range p.Positions {
		found := false
		for _, s := range all {
			if s.Symbol == pos.Symbol {
				found = true
				break
			}
		}
		if !found {
			all = append(all, core.Asset{Symbol: pos.Symbol, Name: pos.Symbol, Market: p.Market, Kind: pos.Kind})
		}
	}
	q := strings.ToLower(strings.TrimSpace(query))
	type match struct {
		asset core.Asset
		score int
	}
	var matches []match
	for _, s := range all {
		if holdingsOnly && m.positionIndex(s.Symbol) < 0 {
			continue
		}
		symbol := strings.ToLower(s.Symbol)
		score, ok := fuzzyScore(q, strings.ToLower(s.Symbol+" "+s.Name))
		if !ok {
			continue
		}
		if q != "" {
			if symbol == q {
				score = -10000
			} else if strings.HasPrefix(symbol, q) {
				score = -5000 + len(symbol)
			}
		}
		matches = append(matches, match{s, score})
	}
	if q != "" || rankPopular {
		sort.SliceStable(matches, func(i, j int) bool {
			if q != "" && matches[i].score != matches[j].score {
				return matches[i].score < matches[j].score
			}
			if rankPopular {
				return m.popularRank(matches[i].asset) < m.popularRank(matches[j].asset)
			}
			return false
		})
	}
	out := make([]core.Asset, 0, len(matches))
	for _, item := range matches {
		out = append(out, item.asset)
	}
	return out
}

func (m Model) activeItems() []core.Asset {
	if m.page == addPage {
		return m.filterAssets(m.add.query, false, true)
	}
	return m.filterAssets(m.holdings.query, m.page == holdingsPage, false)
}

func (m Model) activeCursor() int {
	if m.page == addPage {
		return m.add.cursor
	}
	return m.holdings.cursor
}
