package ui

import (
	"fmt"
	"strings"
)

func (m Model) assetDetails(width int) string {
	a := m.detailAsset
	var b strings.Builder
	b.WriteString(accent.Render(a.Symbol+" / PRICE DETAILS") + "\n" + muted.Render("Esc back · r retry · Ctrl+↑↓ interval · d/w/m/c/y day/week/month/quarter/year") + "\n\n")
	style := cardStyle(width, false)
	bars := m.bars[a.Symbol]
	if len(bars) == 0 {
		state := "Loading price history…"
		if err := m.errors[a.Symbol]; err != "" {
			state = "Price unavailable: " + err
		}
		b.WriteString(style.Render(accent.Render("PRICE INFORMATION") + "\n" + a.Name + "\n" + state))
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
		change = fmt.Sprintf("%+.2f", last.Close-prev) + " (" + coloredPercent((last.Close/prev-1)*100) + ")"
	}
	info := accent.Render("PRICE INFORMATION") + "\n" + a.Name + fmt.Sprintf("\nLast close  %.2f %s\nDaily change  %s\nOpen  %.2f   High  %.2f   Low  %.2f\nDate  %s   Provider  %s", last.Close, m.cfg.Portfolios[0].PriceCurrency, change, last.Open, last.High, last.Low, last.Date.Format("2006-01-02"), last.Source)
	info += "\nEarliest cached history  " + earliest.Format("2006-01-02")
	b.WriteString(style.Render(info) + "\n")
	grain := m.detailGrain
	if grain == "" {
		grain = "day"
	}
	chartWidth := max(20, min(68, width-16))
	visible := candleWindow(aggregateCandles(bars, grain), max(1, (chartWidth-12)/2), m.detailOffset)
	chart := accent.Render("PRICE HISTORY · "+strings.ToUpper(grain)) + "\n\n" + renderCandles(visible, grain, chartWidth, 12)
	b.WriteString(style.Render(chart))
	if err := m.errors[a.Symbol]; err != "" {
		b.WriteString("\nRefresh failed; cached prices shown: " + err)
	}
	return b.String()
}
