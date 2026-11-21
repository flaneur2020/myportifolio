package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/domain"
	"portifolio/internal/ui/theme"
)

func (m *Model) updateAnalysis(key tea.KeyMsg) tea.Cmd {
	k := key.String()
	if m.analysis.confirmDelete {
		switch k {
		case "y", "enter":
			m.removeAnalysisCardAtCursor()
		case "n", "esc":
			m.analysis.confirmDelete = false
			m.status = "Card removal cancelled."
		}
		return nil
	}
	if m.analysis.addMode {
		hidden := m.analysis.hiddenCards(m.cfg.Analysis)
		switch k {
		case "up", "k":
			m.analysis.addCursor = max(0, m.analysis.addCursor-1)
		case "down", "j":
			m.analysis.addCursor = min(max(0, len(hidden)-1), m.analysis.addCursor+1)
		case "enter":
			m.addAnalysisCardAtIndex(m.analysis.addCursor)
		case "esc":
			m.analysis.addMode = false
			m.status = "Add card cancelled."
		}
		return nil
	}

	visible := m.analysis.visibleCards(m.cfg.Analysis)
	selected := m.analysis.cardAt(m.cfg.Analysis, m.analysis.card)

	switch k {
	case "q":
		return tea.Quit
	case "ctrl+up":
		m.analysis.grain = domain.StepGrain(m.analysis.grain, -1)
	case "ctrl+down":
		m.analysis.grain = domain.StepGrain(m.analysis.grain, 1)
	case "x", "delete":
		if len(visible) <= 1 {
			m.status = "At least one Analysis card must remain."
		} else {
			m.analysis.confirmDelete = true
			m.analysis.active = false
			m.status = ""
		}
	case "a", "n":
		m.beginAddAnalysisCard()
	case "J", "shift+down":
		m.moveAnalysisCard(1)
	case "K", "shift+up":
		m.moveAnalysisCard(-1)
	case "pgdown":
		m.analysis.scroll += max(1, m.height-8)
	case "pgup":
		m.analysis.scroll = max(0, m.analysis.scroll-max(1, m.height-8))
	case "tab":
		if m.analysis.active {
			m.analysis.card = min(max(0, len(visible)-1), m.analysis.card+1)
			m.analysis.scroll = 0
			return nil
		}
		m.page = holdingsPage
		m.analysis.active = false
		m.holdings.cursor = 0
		m.holdings.query = ""
	case "1":
		m.page = holdingsPage
		m.analysis.active = false
		m.holdings.cursor = 0
		m.holdings.query = ""
	case "2":
		m.analysis.active = false
	case "esc":
		if m.analysis.active {
			m.analysis.active = false
			return nil
		}
		m.page = holdingsPage
		m.holdings.cursor = 0
		m.holdings.query = ""
	case "up", "k":
		if m.analysis.active {
			if selected != nil && selected.SupportsInterval() {
				m.analysis.grain = domain.StepGrain(m.analysis.grain, -1)
			}
			return nil
		}
		m.analysis.card = max(0, m.analysis.card-1)
		m.analysis.scroll = 0
	case "down", "j":
		if m.analysis.active {
			if selected != nil && selected.SupportsInterval() {
				m.analysis.grain = domain.StepGrain(m.analysis.grain, 1)
			}
			return nil
		}
		m.analysis.card = min(max(0, len(visible)-1), m.analysis.card+1)
		m.analysis.scroll = 0
	case "enter":
		m.analysis.active = true
	case "d":
		m.analysis.active = false
		m.analysis.grain = domain.GrainDay
	case "w":
		m.analysis.grain = domain.GrainWeek
	case "m":
		m.analysis.grain = domain.GrainMonth
	case "c":
		m.analysis.grain = domain.GrainQuarter
	case "y":
		m.analysis.grain = domain.GrainYear
	case "z":
		m.analysis.grain = domain.CycleGrain(m.analysis.grain, 1)
	case "left":
		if m.analysis.active && selected != nil && selected.SupportsPan() {
			m.analysis.shiftOffset(selected.ID(), 1)
		}
	case "right":
		if m.analysis.active && selected != nil && selected.SupportsPan() {
			m.analysis.shiftOffset(selected.ID(), -1)
		}
	case "home":
		if selected != nil {
			m.analysis.setOffset(selected.ID(), 0)
		}
		m.analysis.setOffset("history", 0)
	case "e":
		m.openSettings()
	case "s":
		m.openSettings()
		m.settings.field = 2
	case "b":
		m.cycleRebalance()
	case "r":
		m.requested = map[string]bool{}
		return tea.Batch(m.loadAll(), m.beginDirectory(true))
	}
	return nil
}

func (m Model) viewAnalysis(width int) string {
	p := m.cfg.Portfolio
	heading := theme.Accent.Render("PORTFOLIO ANALYSIS") + "\n" +
		theme.Muted.Render("Esc back · r retry · ↑/↓ select card · Enter activate · z / Ctrl+↑↓ interval · e settings · a add · x remove · J/K move · s start date · b rebalance") + "\n\n"
	infoIndex := m.analysis.visibleIndex(m.cfg.Analysis, "information")
	failure := func(body string) string {
		style := theme.AnalysisCardStyle(width, infoIndex >= 0 && m.analysis.card == infoIndex, m.analysis.active)
		return heading + style.Render(theme.Accent.Render("PORTFOLIO INFORMATION")+"\n"+p.Name+"\n"+body) + m.analysis.pluginErrorsView()
	}

	data, reason := domain.BuildAnalysisData(p, m.bars)
	if reason != "" {
		return failure(reason)
	}
	visible := m.analysis.visibleCards(m.cfg.Analysis)
	if len(visible) == 0 {
		return heading + theme.Muted.Render("No analysis cards are enabled. Press a to add one.") + m.analysis.pluginErrorsView()
	}
	parts := make([]string, 0, len(visible))
	for i, card := range visible {
		ctx := analysisRenderContext{
			data:   data,
			bars:   m.bars,
			grain:  m.analysis.grain,
			offset: m.analysis.offset(card.ID()),
			width:  width,
			style:  theme.AnalysisCardStyle(width, m.analysis.card == i, m.analysis.active),
		}
		parts = append(parts, card.Render(ctx))
	}
	return heading + strings.Join(parts, "\n") + m.analysis.pluginErrorsView()
}

func (m Model) viewAnalysisViewport(width int) string {
	content := m.viewAnalysis(width)
	visible := m.analysis.visibleCards(m.cfg.Analysis)
	selected := m.analysis.cardAt(m.cfg.Analysis, m.analysis.card)

	if m.analysis.confirmDelete || m.analysis.addMode {
		lines := strings.Split(content, "\n")
		prefix := strings.Join(lines[:min(3, len(lines))], "\n")
		var body string
		if m.analysis.confirmDelete {
			title := ""
			if selected != nil {
				title = selected.Title()
			}
			body = theme.Accent.Render("REMOVE CARD") + "\n\n" + title + "\n\n" + theme.Muted.Render("Enter / y remove · Esc / n cancel")
		} else {
			hidden := m.analysis.hiddenCards(m.cfg.Analysis)
			capacity := max(1, min(8, m.height-10))
			start := max(0, m.analysis.addCursor-capacity+1)
			end := min(len(hidden), start+capacity)
			var list strings.Builder
			for i := start; i < end; i++ {
				marker := "  "
				if i == m.analysis.addCursor {
					marker = theme.Highlight.Render("▸ ")
				}
				list.WriteString(marker + hidden[i].Title() + "\n")
			}
			body = theme.Accent.Render("ADD CARD") + "\n\n" + strings.TrimRight(list.String(), "\n") + "\n\n" + theme.Muted.Render("↑/↓ select · Enter add · Esc cancel")
		}
		view := prefix + "\n" + theme.CardStyle(width, true).Render(body)
		if m.status != "" {
			view += "\n" + theme.Muted.Render(m.status)
		}
		return view
	}

	if m.height <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	capacity := max(1, m.height-6)
	heading := ""
	if selected != nil {
		heading = selected.Title()
	}
	anchor := 0
	for i, line := range lines {
		if heading != "" && strings.Contains(line, heading) {
			anchor = max(0, i-1)
			break
		}
	}
	start := max(0, min(anchor+m.analysis.scroll, max(0, len(lines)-capacity)))
	end := min(len(lines), start+capacity)
	total := len(visible)
	status := fmt.Sprintf("Card %d/%d · ↑/↓ select · Enter activate · z interval · PgUp/PgDn scroll", m.analysis.card+1, total)
	if m.analysis.active && selected != nil {
		interval, pan := selected.SupportsInterval(), selected.SupportsPan()
		switch {
		case interval && pan:
			status = fmt.Sprintf("Card %d/%d ACTIVE · ↑/↓ interval · ←/→ pan · Tab next active card · Esc back", m.analysis.card+1, total)
		case interval:
			status = fmt.Sprintf("Card %d/%d ACTIVE · ↑/↓ interval · Tab next active card · Esc back", m.analysis.card+1, total)
		case pan:
			status = fmt.Sprintf("Card %d/%d ACTIVE · ←/→ pan · Tab next active card · Esc back", m.analysis.card+1, total)
		default:
			status = fmt.Sprintf("Card %d/%d ACTIVE · informational card · Esc back to card selection", m.analysis.card+1, total)
		}
	}
	footer := theme.Muted.Render(status)
	if m.status != "" {
		footer += "\n" + theme.Muted.Render(m.status)
	}
	return strings.Join(lines[start:end], "\n") + "\n" + footer
}
