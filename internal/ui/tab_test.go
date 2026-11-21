package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"testing"
)

func TestTabActivatesNextCardOnlyWhileActive(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.page != analysisPage || m.analysisActive {
		t.Fatal("Tab from My Portfolio opens inactive Analysis")
	}
	// Inactive Tab retains the page-switch behaviour.
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.page != holdingsPage || m.analysisActive {
		t.Fatal("inactive Tab must return to My Portfolio")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.analysisActive || m.analysisCard != 0 {
		t.Fatal("Enter did not activate the first card")
	}
	for want := 1; want <= 4; want++ {
		m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
		if m.analysisCard != want || !m.analysisActive {
			t.Fatalf("active Tab should activate card %d, got %d active=%v", want, m.analysisCard, m.analysisActive)
		}
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.analysisCard != 4 || !m.analysisActive {
		t.Fatal("active Tab must clamp at final card")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.analysisActive || m.page != analysisPage {
		t.Fatal("Esc must leave active mode")
	}
	m = pressKey(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.page != holdingsPage {
		t.Fatal("inactive Tab should switch pages again")
	}
}
