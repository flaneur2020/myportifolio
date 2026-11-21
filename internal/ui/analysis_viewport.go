package ui

import (
	"fmt"
	"strings"
)

func (m Model) analysisViewport(width int) string {
	content := m.analysisView(width)
	if m.height <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	capacity := max(1, m.height-6)
	headings := []string{"PORTFOLIO INFORMATION", "PORTFOLIO HISTORY", "DRAWDOWN / HISTORY", "TARGET ALLOCATION", "PERIOD RETURNS"}
	anchor := 0
	for i, line := range lines {
		if strings.Contains(line, headings[m.analysisCard]) {
			anchor = max(0, i-1)
			break
		}
	}
	start := max(0, min(anchor+m.analysisScroll, max(0, len(lines)-capacity)))
	end := min(len(lines), start+capacity)
	status := fmt.Sprintf("Card %d/5 · ↑/↓ select · Enter activate · z interval · PgUp/PgDn scroll", m.analysisCard+1)
	if m.analysisActive {
		if m.analysisCard == 1 || m.analysisCard == 2 || m.analysisCard == 4 {
			status = fmt.Sprintf("Card %d/5 ACTIVE · ↑/↓ interval · ←/→ pan · Tab next active card · Esc back to card selection", m.analysisCard+1)
		} else {
			status = fmt.Sprintf("Card %d/5 ACTIVE · informational card · Esc back to card selection", m.analysisCard+1)
		}
	}
	return strings.Join(lines[start:end], "\n") + "\n" + muted.Render(status)
}
