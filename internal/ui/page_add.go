package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/domain"
	"portifolio/internal/ui/chart"
	"portifolio/internal/ui/theme"
)

type addState struct {
	cursor int
	query  string
}

func (m *Model) updateAdd(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "esc":
		m.page = holdingsPage
		m.add.cursor = 0
		m.add.query = ""
	case "up":
		if m.add.cursor > 0 {
			m.add.cursor--
		}
	case "down":
		if m.add.cursor < len(m.addItems())-1 {
			m.add.cursor++
		}
	case "enter":
		items := m.addItems()
		if m.add.cursor < len(items) {
			if m.positionIndex(items[m.add.cursor].Symbol) < 0 {
				m.toggleFavorite(items, &m.add.cursor)
				if m.positionIndex(items[m.add.cursor].Symbol) < 0 {
					return nil
				}
			}
			m.page = holdingsPage
			m.add.cursor = 0
			m.add.query = ""
		}
	case "backspace":
		m.add.query = trimLast(m.add.query)
		m.add.cursor = 0
	case "ctrl+u":
		m.add.query = ""
		m.add.cursor = 0
	case "ctrl+r":
		return m.beginDirectory(true)
	default:
		if key.Type == tea.KeyRunes {
			m.add.query += string(key.Runes)
			m.add.cursor = 0
		}
	}
	return m.loadAll()
}

func (m Model) addItems() []domain.Asset {
	return m.filterAssets(m.add.query, false, true)
}

func assetSearchInput(query string, width int) string {
	value := query + "▏"
	if query == "" {
		value = "▏ " + theme.Muted.Render("Search symbol or name…")
	}
	return theme.CardStyle(width, true).Render(theme.Muted.Render("SEARCH ASSETS") + "\n" + value)
}

func (m Model) cardHistory(symbol string, width int) string {
	bars := m.bars[symbol]
	if len(bars) > 0 {
		candles := chart.Candles(bars, domain.GrainDay, max(20, min(68, width-16)), 4)
		if m.priceLoading[symbol] {
			return candles + "\n" + m.priceAnimation()
		}
		if m.errors[symbol] != "" {
			return candles + "\n" + theme.Muted.Render("Refresh failed · cached history shown")
		}
		return candles
	}
	if m.priceLoading[symbol] {
		return m.priceAnimation()
	}
	if m.errors[symbol] != "" {
		return theme.Muted.Render("History unavailable · r to retry")
	}
	return theme.Muted.Render(fmt.Sprintf("%s history loads when selected", symbol))
}

func (m Model) cardWindow(count, reserved int) (int, int) {
	capacity := max(1, (m.height-reserved)/13)
	if m.height == 0 {
		capacity = 2
	}
	start := max(0, m.add.cursor-capacity+1)
	return start, min(count, start+capacity)
}

func (m Model) viewAdd(width int) string {
	var b strings.Builder
	b.WriteString(theme.Accent.Render("ADD POSITION") + "\n" + theme.Muted.Render("Type to search · ↑/↓ select · Enter add · Esc back · Ctrl+R refresh") + "\n\n")
	b.WriteString(assetSearchInput(m.add.query, width) + "\n")
	if strings.TrimSpace(m.add.query) == "" {
		b.WriteString(theme.Muted.Render("Common assets first · curated order, not live popularity") + "\n")
	}
	if m.dirLoading {
		b.WriteString(m.directoryProgress(width) + "\n")
	}
	items := m.addItems()
	start, end := m.cardWindow(len(items), 14)
	if len(items) == 0 {
		message := "No matching assets. Try another symbol or name."
		if m.dirLoading {
			message = "Loading asset directory… You can search while waiting."
		}
		b.WriteString(theme.CardStyle(width, false).Render(message) + "\n")
	}
	for i := start; i < end; i++ {
		a := items[i]
		prefix := "  "
		if i == m.add.cursor {
			prefix = "▸ "
		}
		state := "Enter to add"
		if m.positionIndex(a.Symbol) >= 0 {
			state = "✓ In My Portfolio"
		}
		price := "Price not loaded"
		bars := m.bars[a.Symbol]
		if len(bars) > 0 {
			price = fmt.Sprintf("Last %.2f %s", bars[len(bars)-1].Close, m.cfg.Portfolio.PriceCurrency)
		}
		name := truncateText(a.Name, max(12, min(60, width-16)))
		body := fmt.Sprintf("%s%s · %s\n%s\n%s · %s", prefix, a.Symbol, strings.ToUpper(a.Kind), theme.Muted.Render(name), price, state)
		if len(bars) > 1 && bars[len(bars)-2].Close > 0 {
			body += " · Day " + theme.ColoredPercent((bars[len(bars)-1].Close/bars[len(bars)-2].Close-1)*100)
		}
		body += "\n\n" + m.cardHistory(a.Symbol, width)
		b.WriteString(theme.CardStyle(width, i == m.add.cursor).Render(body) + "\n")
	}
	if len(items) > 0 {
		b.WriteString(theme.Muted.Render(fmt.Sprintf("Results %d–%d of %d · ↑/↓ scroll", start+1, end, len(items))) + "\n")
	}
	b.WriteString("\n" + m.dirStatus)
	return b.String()
}
