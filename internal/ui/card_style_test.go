package ui

import (
	"fmt"
	"portifolio/internal/config"
	"strings"
	"testing"
)

func TestAnalysisCardSelectedAndActiveStylesDiffer(t *testing.T) {
	c, _ := config.New("us")
	m := New(&c, "", nil, nil)
	m.page = analysisPage
	m.analysisCard = 0
	selected := m.analysisCardStyle(80, 0)
	inactive := m.analysisCardStyle(80, 1)
	if fmt.Sprint(selected.GetBorderTopForeground()) == fmt.Sprint(inactive.GetBorderTopForeground()) {
		t.Fatal("selected card must use a distinct highlight colour")
	}
	if strings.Contains(selected.Render("card"), "╔") {
		t.Fatal("inactive-selected card must not use active border")
	}
	m.analysisActive = true
	active := m.analysisCardStyle(80, 0)
	if fmt.Sprint(active.GetBorderStyle()) == fmt.Sprint(selected.GetBorderStyle()) {
		t.Fatal("active card must use a distinct border style")
	}
	if !strings.Contains(active.Render("card"), "╔") || !strings.Contains(active.Render("card"), "╗") {
		t.Fatal("active card must render a double border")
	}
	if fmt.Sprint(m.analysisCardStyle(80, 1).GetBorderTopForeground()) != fmt.Sprint(inactive.GetBorderTopForeground()) {
		t.Fatal("inactive cards must remain unchanged")
	}
}
