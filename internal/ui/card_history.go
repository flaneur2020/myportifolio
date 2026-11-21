package ui

import "fmt"

// Reserve space for chart axes and card padding so the chart stays inside its border.
func (m Model) cardHistory(symbol string, width int) string {
	bars := m.bars[symbol]
	if len(bars) > 0 {
		chart := renderCandles(bars, "day", max(20, min(68, width-16)), 4)
		if m.priceLoading[symbol] {
			return chart + "\n" + m.priceAnimation()
		}
		if m.errors[symbol] != "" {
			return chart + "\n" + muted.Render("Refresh failed · cached history shown")
		}
		return chart
	}
	if m.priceLoading[symbol] {
		return m.priceAnimation()
	}
	if m.errors[symbol] != "" {
		return muted.Render("History unavailable · r to retry")
	}
	return muted.Render(fmt.Sprintf("%s history loads when selected", symbol))
}

func (m Model) cardWindow(count, reserved int) (int, int) {
	capacity := max(1, (m.height-reserved)/13)
	if m.height == 0 {
		capacity = 2
	}
	start := max(0, m.cursor-capacity+1)
	return start, min(count, start+capacity)
}
