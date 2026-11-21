package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The add card is a real list item, including when the portfolio is empty.
func (m Model) holdingsCards(width int) string {
	var b strings.Builder
	b.WriteString(accent.Render("MY PORTFOLIO") + "\n" + muted.Render("↑/↓ select card · Enter/d details · e target weight · a Add · x remove") + "\n\n")
	items := m.items()
	count := len(items) + 1
	// Summary cards occupy six lines including their border and spacing.
	capacity := max(1, (m.height-10)/6)
	if m.height == 0 {
		capacity = 5
	}
	start := max(0, m.cursor-capacity+1)
	end := min(count, start+capacity)
	for i := start; i < end; i++ {
		style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#3d4d59")).Padding(0, 2).Width(max(18, min(76, width-8)))
		prefix := "  "
		if i == m.cursor {
			style = style.BorderForeground(lipgloss.Color("#ffd166"))
			prefix = "▸ "
		}
		var body string
		if i == len(items) {
			body = prefix + "+ Add position\n\n" + muted.Render("Search the asset directory and add to your portfolio")
		} else {
			asset := items[i]
			idx := m.positionIndex(asset.Symbol)
			weight := m.cfg.Portfolios[0].Positions[idx].Allocation * 100
			price, change := "—", "—"
			bars := m.bars[asset.Symbol]
			if len(bars) > 0 {
				price = fmt.Sprintf("%.2f %s", bars[len(bars)-1].Close, m.cfg.Portfolios[0].PriceCurrency)
				if len(bars) > 1 && bars[len(bars)-2].Close > 0 {
					change = coloredPercent((bars[len(bars)-1].Close/bars[len(bars)-2].Close - 1) * 100)
				}
			}
			name := asset.Name
			maxName := max(12, min(60, width-16))
			runes := []rune(name)
			if len(runes) > maxName {
				name = string(runes[:maxName-1]) + "…"
			}
			body = fmt.Sprintf("%s%s · %s\n%s\nLast %s   Day %s   Target %.1f%%", prefix, asset.Symbol, strings.ToUpper(asset.Kind), muted.Render(name), price, change, weight)
		}
		b.WriteString(style.Render(body) + "\n")
	}
	if start > 0 || end < count {
		b.WriteString(muted.Render(fmt.Sprintf("Cards %d–%d of %d · ↑/↓ scroll", start+1, end, count)) + "\n")
	}
	p := m.cfg.Portfolios[0]
	b.WriteString(fmt.Sprintf("\n%d positions · target total %.1f%% · rebalance %s", len(p.Positions), totalAllocation(p.Positions)*100, p.Rebalance))
	if m.searching || m.query != "" {
		b.WriteString("\nSearch: " + m.query + "▏")
	}
	if m.status != "" {
		b.WriteString("\n" + m.status)
	}
	return b.String()
}
