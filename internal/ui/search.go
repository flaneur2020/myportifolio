package ui

import (
	"sort"
	"strings"

	"portifolio/internal/config"
	"portifolio/internal/domain"
)

// Curated common assets, not a real-time popularity feed. Only assets already
// returned by the directory are ranked; this never invents directory entries.
var commonAssets = map[string][]string{
	config.MarketUS: {"SPY", "QQQ", "VTI", "VOO", "AAPL", "MSFT", "NVDA", "AMZN", "GOOGL", "META", "TSLA", "IWM", "DIA", "BND", "TLT", "GLD"},
	config.MarketCN: {"510300", "510500", "510050", "159915", "588000", "159919", "512100", "518880", "600519", "600036", "601318", "000858", "300750"},
}

func popularRank(asset domain.Asset) int {
	for i, symbol := range commonAssets[asset.Market] {
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

func (m Model) filterAssets(query string, holdingsOnly, rankPopular bool) []domain.Asset {
	p := m.cfg.Portfolio
	if p.Market == "" {
		return nil
	}
	var all []domain.Asset
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
			all = append(all, domain.Asset{Symbol: pos.Symbol, Name: pos.Symbol, Market: p.Market, Kind: pos.Kind})
		}
	}
	q := strings.ToLower(strings.TrimSpace(query))
	type match struct {
		asset domain.Asset
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
				return popularRank(matches[i].asset) < popularRank(matches[j].asset)
			}
			return false
		})
	}
	out := make([]domain.Asset, 0, len(matches))
	for _, item := range matches {
		out = append(out, item.asset)
	}
	return out
}

func (m Model) activeItems() []domain.Asset {
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
