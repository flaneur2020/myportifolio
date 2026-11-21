package chart

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"portifolio/internal/domain"
)

func TestCandlesRender(t *testing.T) {
	bars := []domain.Bar{
		{Date: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC), Open: 100, High: 110, Low: 95, Close: 105},
		{Date: time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC), Open: 105, High: 115, Low: 99, Close: 101},
		{Date: time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC), Open: 101, High: 103, Low: 90, Close: 92},
	}
	view := Candles(bars, domain.GrainDay, 60, 10)
	for _, label := range []string{"│", "█"} {
		if !strings.Contains(view, label) {
			t.Fatalf("chart missing %q", label)
		}
	}
	if strings.Contains(view, "•") {
		t.Fatal("chart still uses dots")
	}
}

func TestReturnBarsDirectionAndAxis(t *testing.T) {
	curve := []domain.Point{
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Value: 100},
		{Date: time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), Value: 120},
		{Date: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), Value: 90},
		{Date: time.Date(2024, 3, 29, 0, 0, 0, 0, time.UTC), Value: 99},
	}
	plain := ansi.Strip(ReturnBars(domain.PeriodReturns(curve, domain.GrainMonth), 54, 7, 10000, "USD"))
	lines := strings.Split(plain, "\n")
	zero, up, down := -1, -1, -1
	for i, line := range lines {
		if zero < 0 && strings.Contains(line, "$0") {
			zero = i
			break
		}
	}
	for i, line := range lines {
		if up < 0 && i < zero && strings.Contains(line, "━━") {
			up = i
		}
		if down < 0 && i > zero && strings.Contains(line, "━━") {
			down = i
		}
	}
	if zero < 0 || up < 0 || down < 0 || !(up < zero && zero < down) {
		t.Fatalf("gains must be above zero and losses below: up=%d zero=%d down=%d\n%s", up, zero, down, plain)
	}
	if !strings.Contains(plain, "+$2.5K") || !strings.Contains(plain, "2024-03-29") {
		t.Fatalf("latest date missing from axis:\n%s", plain)
	}
}

func TestDrawdownFlatAndNormal(t *testing.T) {
	points := []domain.Point{
		{Date: time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), Value: -20},
		{Date: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), Value: -3},
	}
	view := Drawdown(points, 40, 7)
	for _, want := range []string{"0.00%", "-20.00%", "└", "2024-01-31", "░", "━"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q: %s", want, view)
		}
	}
	flat := Drawdown([]domain.Point{{Date: time.Now(), Value: 0}}, 20, 7)
	if strings.Contains(flat, "NaN") || strings.Contains(flat, "Inf") {
		t.Fatal("flat history produced invalid axis")
	}
}

func TestStackedWeightsAndDriftHistory(t *testing.T) {
	bar := ansi.Strip(StackedWeights([]float64{.6, .3, .1}, 40))
	if len([]rune(bar)) != 40 || !strings.Contains(bar, "█") {
		t.Fatalf("bad stacked bar %q", bar)
	}
	points := []domain.Point{
		{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Value: 5},
		{Date: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), Value: 12},
	}
	out := ansi.Strip(DriftHistory(points, 40, 4))
	for _, want := range []string{"12.0% │", "0.0% │", "└", "2024-01-01 → 2024-02-01"} {
		if !strings.Contains(out, want) {
			t.Fatalf("drift history missing %q:\n%s", want, out)
		}
	}
}
