package ui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

func cardStyle(width int, focused bool) lipgloss.Style {
	color := "#3d4d59"
	if focused {
		color = "#ffd166"
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(color)).Padding(0, 2).Width(max(18, min(76, width-8)))
}
func assetSearchInput(query string, width int) string {
	value := query + "▏"
	if query == "" {
		value = "▏ " + muted.Render("Search symbol or name…")
	}
	return cardStyle(width, true).Render(muted.Render("SEARCH ASSETS") + "\n" + value)
}
func (m Model) addCards(width int) string {
	var b strings.Builder
	b.WriteString(accent.Render("ADD POSITION") + "\n" + muted.Render("Type to search · ↑/↓ select · Enter add · Esc back · Ctrl+R refresh") + "\n\n")
	b.WriteString(assetSearchInput(m.query, width) + "\n")
	if strings.TrimSpace(m.query) == "" {
		b.WriteString(muted.Render("Common assets first · curated order, not live popularity") + "\n")
	}
	if m.directoryLoading {
		b.WriteString(m.directoryProgress(width) + "\n")
	}
	items := m.items()
	start, end := m.cardWindow(len(items), 14)
	if len(items) == 0 {
		message := "No matching assets. Try another symbol or name."
		if m.directoryLoading {
			message = "Loading asset directory… You can search while waiting."
		}
		b.WriteString(cardStyle(width, false).Render(message) + "\n")
	}
	for i := start; i < end; i++ {
		a := items[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		state := "Enter to add"
		if m.positionIndex(a.Symbol) >= 0 {
			state = "✓ In My Portfolio"
		}
		price := "Price not loaded"
		bars := m.bars[a.Symbol]
		if len(bars) > 0 {
			price = fmt.Sprintf("Last %.2f %s", bars[len(bars)-1].Close, m.cfg.Portfolios[0].PriceCurrency)
		}
		name := a.Name
		limit := max(12, min(60, width-16))
		runes := []rune(name)
		if len(runes) > limit {
			name = string(runes[:limit-1]) + "…"
		}
		body := fmt.Sprintf("%s%s · %s\n%s\n%s · %s", prefix, a.Symbol, strings.ToUpper(a.Kind), muted.Render(name), price, state)
		if len(bars) > 1 && bars[len(bars)-2].Close > 0 {
			body += " · Day " + coloredPercent((bars[len(bars)-1].Close/bars[len(bars)-2].Close-1)*100)
		}
		body += "\n\n" + m.cardHistory(a.Symbol, width)
		b.WriteString(cardStyle(width, i == m.cursor).Render(body) + "\n")
	}
	if len(items) > 0 {
		b.WriteString(muted.Render(fmt.Sprintf("Results %d–%d of %d · ↑/↓ scroll", start+1, end, len(items))) + "\n")
	}

	b.WriteString("\n" + m.directoryStatus)
	return b.String()
}
