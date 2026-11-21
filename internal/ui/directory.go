package ui

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"time"
)

func (m Model) loadDirectory(refresh bool) tea.Cmd {
	if m.store == nil || len(m.cfg.Portfolios) == 0 {
		return nil
	}
	market := m.cfg.Portfolios[0].Market
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if !refresh {
			assets, updated, err := m.store.Assets(ctx, market)
			if err == nil && len(assets) > 0 {
				return directoryLoaded{assets: assets, status: "Asset directory cached · " + updated.Format(time.RFC3339), refresh: time.Since(updated) > 24*time.Hour}
			}
		}
		assets, err := m.directory.Assets(ctx, market)
		if err != nil {
			return directoryLoaded{status: "Directory refresh failed; cached assets retained: " + err.Error()}
		}
		if err = m.store.PutAssets(ctx, market, assets); err != nil {
			return directoryLoaded{status: "Directory cache write failed: " + err.Error()}
		}
		return directoryLoaded{assets: assets, status: "API asset directory refreshed · " + time.Now().Format(time.RFC3339)}
	}
}

func directoryTickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return directoryTick{} })
}
func (m *Model) beginDirectory(refresh bool) tea.Cmd {
	if m.directoryLoading {
		return nil
	}
	cmd := m.loadDirectory(refresh)
	if cmd == nil {
		return nil
	}
	m.directoryLoading = true
	m.directoryFrame = 0
	m.directoryStatus = "Reading cached asset directory…"
	if refresh {
		m.directoryStatus = "Refreshing asset directory from API…"
	}
	return tea.Batch(cmd, directoryTickCmd())
}
func (m Model) directoryProgress(width int) string {
	size := max(10, min(36, width-12))
	position := m.directoryFrame%(size+6) - 5
	var bar strings.Builder
	for i := 0; i < size; i++ {
		if i >= position && i < position+6 {
			bar.WriteString("━")
		} else {
			bar.WriteString("─")
		}
	}
	return accent.Render("["+bar.String()+"]") + "  Loading asset directory…"
}
