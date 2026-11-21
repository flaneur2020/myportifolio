package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/internal/config"
	"portifolio/internal/domain"
	"portifolio/internal/ui/chart"
	"portifolio/internal/ui/theme"
)

type detailState struct {
	asset  domain.Asset
	grain  domain.Grain
	offset int
}

func (m *Model) updateDetail(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "ctrl+up", "up":
		return m.changeDetailRange(domain.StepGrain(m.detail.grain, -1))
	case "ctrl+down", "down":
		return m.changeDetailRange(domain.StepGrain(m.detail.grain, 1))
	case "z":
		return m.changeDetailRange(domain.CycleGrain(m.detail.grain, 1))
	case "left":
		m.detail.offset++
	case "right":
		m.detail.offset = max(0, m.detail.offset-1)
	case "home":
		m.detail.offset = 0
	case "esc", "q":
		m.page = holdingsPage
	case "d":
		return m.changeDetailRange(domain.GrainDay)
	case "w":
		return m.changeDetailRange(domain.GrainWeek)
	case "m":
		return m.changeDetailRange(domain.GrainMonth)
	case "c":
		return m.changeDetailRange(domain.GrainQuarter)
	case "y":
		return m.changeDetailRange(domain.GrainYear)
	case "r":
		delete(m.requested, m.detail.asset.Symbol)
		return m.loadAll()
	}
	return nil
}

// Asset history is independent of the portfolio's simulation start date.
func (m *Model) changeDetailRange(grain domain.Grain) tea.Cmd {
	m.detail.grain = grain
	if !m.data.CanLoadPrices() || m.priceLoading[m.detail.asset.Symbol] {
		return nil
	}
	asset := m.detail.asset
	key := asset.Symbol + ":" + string(grain)
	if m.rangeRequested == nil {
		m.rangeRequested = map[string]bool{}
	}
	if m.rangeRequested[key] {
		return nil
	}
	end := time.Now()
	start := domain.RangeStart(end, grain)
	cached := m.bars[asset.Symbol]
	if len(cached) > 0 && !cached[0].Date.After(start.AddDate(0, 0, 7)) {
		return nil
	}
	wasLoading := len(m.priceLoading) > 0
	if m.priceLoading == nil {
		m.priceLoading = map[string]bool{}
	}
	m.priceLoading[asset.Symbol] = true
	m.rangeRequested[key] = true
	cfg := m.cfg.Clone()
	svc := m.data
	cmd := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		pos := config.Position{Symbol: asset.Symbol, Market: cfg.Portfolio.Market, Currency: cfg.Portfolio.PriceCurrency, Kind: asset.Kind}
		bars, err := svc.LoadHistory(ctx, cfg.Markets, pos, start, end, true, cached)
		return assetLoaded{symbol: asset.Symbol, bars: bars, err: err}
	}
	if !wasLoading {
		return tea.Batch(cmd, priceTickCmd())
	}
	return cmd
}

func (m Model) viewDetail(width int) string {
	a := m.detail.asset
	var b strings.Builder
	b.WriteString(theme.Accent.Render(a.Symbol+" / PRICE DETAILS") + "\n" + theme.Muted.Render("Esc back · r retry · Ctrl+↑↓ interval · d/w/m/c/y day/week/month/quarter/year") + "\n\n")
	style := theme.CardStyle(width, false)
	bars := m.bars[a.Symbol]
	if len(bars) == 0 {
		state := "Loading price history…"
		if err := m.errors[a.Symbol]; err != "" {
			state = "Price unavailable: " + err
		}
		b.WriteString(style.Render(theme.Accent.Render("PRICE INFORMATION") + "\n" + a.Name + "\n" + state))
		return b.String()
	}
	earliest := bars[0].Date
	for _, bar := range bars {
		if bar.Date.Before(earliest) {
			earliest = bar.Date
		}
	}
	last := bars[len(bars)-1]
	change := "—"
	if len(bars) > 1 && bars[len(bars)-2].Close > 0 {
		prev := bars[len(bars)-2].Close
		change = fmt.Sprintf("%+.2f", last.Close-prev) + " (" + theme.ColoredPercent((last.Close/prev-1)*100) + ")"
	}
	info := theme.Accent.Render("PRICE INFORMATION") + "\n" + a.Name + fmt.Sprintf(
		"\nLast close  %.2f %s\nDaily change  %s\nOpen  %.2f   High  %.2f   Low  %.2f\nDate  %s   Provider  %s",
		last.Close, m.cfg.Portfolio.PriceCurrency, change, last.Open, last.High, last.Low, last.Date.Format("2006-01-02"), last.Source,
	)
	info += "\nEarliest cached history  " + earliest.Format("2006-01-02")
	b.WriteString(style.Render(info) + "\n")

	grain := m.detail.grain
	if grain == "" {
		grain = domain.GrainDay
	}
	chartWidth := max(20, min(68, width-16))
	visible := domain.CandleWindow(domain.AggregateCandles(bars, grain), max(1, (chartWidth-12)/2), m.detail.offset)
	candles := theme.Accent.Render("PRICE HISTORY · "+strings.ToUpper(string(grain))) + "\n\n" + chart.Candles(visible, grain, chartWidth, 12)
	b.WriteString(style.Render(candles))
	if err := m.errors[a.Symbol]; err != "" {
		b.WriteString("\nRefresh failed; cached prices shown: " + err)
	}
	return b.String()
}
