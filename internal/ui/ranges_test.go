package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"testing"
	"time"
)

func TestIntervalRangesExpand(t *testing.T) {
	end := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	previous := end
	for _, grain := range []string{"day", "week", "month", "quarter"} {
		start := rangeStart(end, grain)
		if !start.Before(previous) {
			t.Fatalf("%s did not expand history", grain)
		}
		previous = start
	}
}
func TestAnalysisKeysChangeRange(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m.page = analysisPage
	for _, tc := range []struct {
		key   rune
		grain string
		days  int
	}{{'d', "day", 90}, {'w', "week", 365}, {'m', "month", 1825}, {'c', "quarter", 5475}} {
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
		if m.grain != tc.grain || m.window != tc.days {
			t.Fatalf("%c: %s %d", tc.key, m.grain, m.window)
		}
	}
}
