package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"portifolio/internal/ui/theme"
)

type holdingsState struct {
	cursor    int
	query     string
	searching bool
}

func (m *Model) updateHoldings(key tea.KeyMsg) tea.Cmd {
	k := key.String()
	if m.holdings.searching {
		switch k {
		case "esc", "enter":
			m.holdings.searching = false
		case "backspace":
			m.holdings.query = trimLast(m.holdings.query)
			m.holdings.cursor = 0
		default:
			if key.Type == tea.KeyRunes {
				m.holdings.query += string(key.Runes)
				m.holdings.cursor = 0
			}
		}
		return nil
	}
	switch k {
	case "q":
		return tea.Quit
	case "tab", "2":
		m.page = analysisPage
		m.analysis.active = false
		m.holdings.cursor = 0
		m.holdings.query = ""
	case "1":
		m.holdings.cursor = 0
		m.holdings.query = ""
	case "a", "n":
		return m.openAddPage()
	case "/":
		m.holdings.searching = true
	case "esc":
		m.holdings.query = ""
		m.holdings.cursor = 0
	case "up", "k":
		if m.holdings.cursor > 0 {
			m.holdings.cursor--
		}
	case "down", "j":
		if m.holdings.cursor < len(m.holdingsItems()) {
			m.holdings.cursor++
		}
	case "x", "delete":
		return m.toggleFavorite(m.holdingsItems(), &m.holdings.cursor)
	case "e":
		items := m.holdingsItems()
		if m.holdings.cursor < len(items) {
			if idx := m.positionIndex(items[m.holdings.cursor].Symbol); idx >= 0 {
				symbol := items[m.holdings.cursor].Symbol
				m.weight.symbol = symbol
				m.weight.input = strconv.FormatFloat(m.cfg.Portfolio.Positions[idx].Allocation*100, 'f', -1, 64)
				m.status = symbol
				m.page = weightPage
			}
		}
	case "s":
		m.openSettings()
		m.settings.field = 2
	case "b":
		m.cycleRebalance()
	case "r":
		m.requested = map[string]bool{}
		return tea.Batch(m.loadAll(), m.beginDirectory(true))
	case "d", "enter":
		items := m.holdingsItems()
		if m.holdings.cursor == len(items) {
			if k == "enter" {
				return m.openAddPage()
			}
			return nil
		}
		if m.holdings.cursor < len(items) {
			m.detail.asset = items[m.holdings.cursor]
			m.detail.grain = domain.GrainDay
			m.detail.offset = 0
			cmd := m.loadAll()
			m.page = detailPage
			return tea.Batch(cmd, m.changeDetailRange(domain.GrainDay))
		}
	}
	return m.loadAll()
}

func (m Model) holdingsItems() []domain.Asset {
	return m.filterAssets(m.holdings.query, true, false)
}

func (m *Model) toggleFavorite(items []domain.Asset, cursor *int) tea.Cmd {
	if *cursor >= len(items) {
		return nil
	}
	s := items[*cursor]
	c := m.cfg.Clone()
	p := &c.Portfolio
	idx := m.positionIndex(s.Symbol)
	if idx >= 0 {
		p.Positions = append(p.Positions[:idx], p.Positions[idx+1:]...)
	} else {
		p.Positions = append(p.Positions, config.Position{Symbol: s.Symbol, Market: p.Market, Kind: s.Kind, Currency: p.PriceCurrency})
	}
	if err := config.Save(m.cfgPath, c); err != nil {
		m.status = err.Error()
		return nil
	}
	m.cfg = c
	m.status = "Favorites saved · press e to set target weight"
	remaining := m.activeItems()
	if *cursor >= len(remaining) {
		*cursor = max(0, len(remaining)-1)
	}
	return nil
}

// The add card is a real list item, including when the portfolio is empty.
func (m Model) viewHoldings(width int) string {
	var b strings.Builder
	b.WriteString(theme.Accent.Render("MY PORTFOLIO") + "\n" + theme.Muted.Render("↑/↓ select card · Enter/d details · e target weight · a Add · x remove") + "\n\n")
	items := m.holdingsItems()
	count := len(items) + 1
	// Summary cards occupy six lines including their border and spacing.
	capacity := max(1, (m.height-10)/6)
	if m.height == 0 {
		capacity = 5
	}
	start := max(0, m.holdings.cursor-capacity+1)
	end := min(count, start+capacity)
	p := m.cfg.Portfolio
	for i := start; i < end; i++ {
		focused := i == m.holdings.cursor
		marker := "  "
		if focused {
			marker = "▸ "
		}
		var body string
		if i == len(items) {
			body = marker + "+ Add position\n\n  " + theme.Muted.Render("Search the asset directory and add to your portfolio")
		} else {
			asset := items[i]
			idx := m.positionIndex(asset.Symbol)
			weight := p.Positions[idx].Allocation * 100
			price, change := "—", "—"
			bars := m.bars[asset.Symbol]
			if len(bars) > 0 {
				price = fmt.Sprintf("%.2f %s", bars[len(bars)-1].Close, p.PriceCurrency)
				if len(bars) > 1 && bars[len(bars)-2].Close > 0 {
					change = theme.ColoredPercent((bars[len(bars)-1].Close/bars[len(bars)-2].Close - 1) * 100)
				}
			}
			name := truncateText(asset.Name, max(12, min(60, width-16)))
			body = fmt.Sprintf("%s%s · %s\n  %s\n  Last %s   Day %s   Target %.1f%%",
				marker, asset.Symbol, strings.ToUpper(asset.Kind), theme.Muted.Render(name), price, change, weight)
		}
		b.WriteString(theme.CardStyle(width, focused).Render(body) + "\n")
	}
	if start > 0 || end < count {
		b.WriteString(theme.Muted.Render(fmt.Sprintf("Cards %d–%d of %d · ↑/↓ scroll", start+1, end, count)) + "\n")
	}
	b.WriteString(fmt.Sprintf("\n%d positions · target total %.1f%% · rebalance %s", len(p.Positions), domain.TotalAllocation(p.Positions)*100, p.Rebalance))
	if m.holdings.searching || m.holdings.query != "" {
		b.WriteString("\nSearch: " + m.holdings.query + "▏")
	}
	if m.status != "" {
		b.WriteString("\n" + m.status)
	}
	return b.String()
}
