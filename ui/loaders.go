package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/core"
	"portifolio/ui/theme"
)

func priceTickCmd() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg { return priceTick{} })
}

func directoryTickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return directoryTick{} })
}

func (m Model) priceAnimation() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return theme.Accent.Render(frames[m.priceFrame%len(frames)]) + fmt.Sprintf(" Loading price history… (%d assets)", len(m.priceLoading))
}

func (m Model) directoryProgress(width int) string {
	size := max(10, min(36, width-12))
	position := m.dirFrame%(size+6) - 5
	var bar strings.Builder
	for i := 0; i < size; i++ {
		if i >= position && i < position+6 {
			bar.WriteString("━")
		} else {
			bar.WriteString("─")
		}
	}
	return theme.Accent.Render("["+bar.String()+"]") + "  Loading asset directory…"
}

func (m *Model) loadAll() tea.Cmd {
	if !m.kernel.CanLoadPrices() {
		return nil
	}
	var cmds []tea.Cmd
	wasLoading := len(m.priceLoading) > 0
	if m.priceLoading == nil {
		m.priceLoading = map[string]bool{}
	}
	if m.requested == nil {
		m.requested = map[string]bool{}
	}
	items := m.activeItems()
	cursor := m.activeCursor()
	var selected []core.Asset
	for i, s := range items {
		if i == cursor || m.positionIndex(s.Symbol) >= 0 {
			selected = append(selected, s)
		}
	}
	for _, s := range selected {
		if m.requested[s.Symbol] || m.priceLoading[s.Symbol] {
			continue
		}
		m.requested[s.Symbol] = true
		m.priceLoading[s.Symbol] = true
		asset := s
		cfg := m.cfg.Clone()
		kernel := m.kernel
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			p := cfg.Portfolio
			start, _ := time.Parse("2006-01-02", p.StartDate)
			pos := core.Position{Symbol: asset.Symbol, Market: p.Market, Kind: asset.Kind, Currency: p.PriceCurrency}
			bars, err := kernel.LoadHistory(ctx, pos, start, time.Now(), false, nil)
			return assetLoaded{symbol: asset.Symbol, bars: bars, err: err}
		})
	}
	if !wasLoading && len(m.priceLoading) > 0 {
		cmds = append(cmds, priceTickCmd())
	}
	return tea.Batch(cmds...)
}

func (m Model) loadDirectory(refresh bool) tea.Cmd {
	if !m.kernel.CanLoadDirectory() || m.cfg.Portfolio.Market == "" {
		return nil
	}
	market := m.cfg.Portfolio.Market
	kernel := m.kernel
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		res := kernel.LoadDirectory(ctx, market, refresh)
		return directoryLoaded{assets: res.Assets, status: res.Status, refresh: res.NeedsRefresh}
	}
}

func (m *Model) beginDirectory(refresh bool) tea.Cmd {
	if m.dirLoading {
		return nil
	}
	cmd := m.loadDirectory(refresh)
	if cmd == nil {
		return nil
	}
	m.dirLoading = true
	m.dirFrame = 0
	m.dirStatus = "Reading cached asset directory…"
	if refresh {
		m.dirStatus = "Refreshing asset directory from API…"
	}
	return tea.Batch(cmd, directoryTickCmd())
}
