package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"portifolio/internal/domain"
	"portifolio/internal/ui/chart"
	"portifolio/internal/ui/theme"
)

type builtinCard struct {
	id               string
	title            string
	supportsInterval bool
	supportsPan      bool
	render           func(ctx analysisRenderContext) string
}

func (c builtinCard) ID() string                               { return c.id }
func (c builtinCard) Title() string                            { return c.title }
func (c builtinCard) SupportsInterval() bool                   { return c.supportsInterval }
func (c builtinCard) SupportsPan() bool                        { return c.supportsPan }
func (c builtinCard) Render(ctx analysisRenderContext) string  { return c.render(ctx) }

func builtinAnalysisCards() []analysisCard {
	return []analysisCard{
		builtinCard{id: "information", title: "PORTFOLIO INFORMATION", render: renderInformationCard},
		builtinCard{id: "history", title: "PORTFOLIO HISTORY", supportsInterval: true, supportsPan: true, render: renderHistoryCard},
		builtinCard{id: "drawdown", title: "DRAWDOWN / HISTORY", supportsInterval: true, supportsPan: true, render: renderDrawdownCard},
		builtinCard{id: "allocation", title: "TARGET ALLOCATION", render: renderAllocationCard},
		builtinCard{id: "returns", title: "PERIOD RETURNS", supportsInterval: true, supportsPan: true, render: renderPeriodReturnsCard},
		builtinCard{id: "drift", title: "WEIGHT DRIFT", supportsInterval: true, supportsPan: true, render: renderWeightDriftCard},
		builtinCard{id: "contribution", title: "ASSET CONTRIBUTION", supportsInterval: true, supportsPan: true, render: renderContributionCard},
	}
}

func renderInformationCard(ctx analysisRenderContext) string {
	d := ctx.data
	p := d.Portfolio
	latest := d.Curve[len(d.Curve)-1]
	recent := "—"
	if percent, amount, ok := domain.RecentChange(d.Curve, d.Capital); ok {
		recent = latest.Date.Format("2006-01-02") + "  " + theme.ColoredPercent(percent) + "  " + theme.ColoredAmount(amount, p.BaseCurrency)
	}
	rows := [][2]string{
		{"Model index", fmt.Sprintf("%.2f", latest.Value)},
		{"Total return", theme.ColoredPercent(latest.Value - 100)},
		{"Max drawdown", theme.ColoredPercent(d.MaxDrawdown * 100)},
		{"Recent day", recent},
		{},
		{"Requested start", p.StartDate},
		{"Effective start", d.Curve[0].Date.Format("2006-01-02")},
		{"Latest data", latest.Date.Format("2006-01-02")},
		{"Rebalance", p.Rebalance},
		{"Initial capital", fmt.Sprintf("%.2f %s", d.Capital, p.BaseCurrency)},
		{"Model value", fmt.Sprintf("%.2f %s", d.Capital*latest.Value/100, p.BaseCurrency)},
	}
	weightNote := fmt.Sprintf("Configured weight total %.1f%% · normalized to 100%% for analysis", d.ConfiguredWeight*100)
	summary := theme.RenderMetricRows(rows, 17) + "\n" + theme.Muted.Render(weightNote)
	if d.EffectiveStartNote != "" {
		summary += "\n" + theme.Muted.Render(d.EffectiveStartNote)
	}
	return ctx.style.Render(theme.Accent.Render("PORTFOLIO INFORMATION") + "\n" + p.Name + "\n" + summary)
}

func renderHistoryCard(ctx analysisRenderContext) string {
	curve := ctx.data.Curve
	all := domain.AggregateCandles(domain.PortfolioCandles(curve), ctx.grain)
	chartWidth := max(20, min(68, ctx.width-16))
	capacity := max(1, (chartWidth-12)/2)
	visible := domain.CandleWindow(all, capacity, ctx.offset)
	if len(visible) == 0 {
		return "No portfolio history available"
	}
	coverageStart := visible[0].Date
	periodStart := domain.PeriodStart(coverageStart, ctx.grain)
	for _, point := range curve {
		if !point.Date.Before(periodStart) && !point.Date.After(visible[0].Date) {
			coverageStart = point.Date
			break
		}
	}
	return ctx.style.Render(
		theme.Accent.Render("PORTFOLIO HISTORY · "+string(ctx.grain)) + "\n\n" +
			chart.Candles(visible, domain.GrainDay, chartWidth, 12) + "\n" +
			theme.Muted.Render(fmt.Sprintf("Window: %s → %s", coverageStart.Format("2006-01-02"), visible[len(visible)-1].Date.Format("2006-01-02"))) + "\n" +
			theme.Muted.Render("←/→ move one candle · z day/week/month/quarter/year") + "\n" +
			theme.Muted.Render("Model candles use daily closes; wicks are not intraday highs/lows."),
	)
}

func renderDrawdownCard(ctx analysisRenderContext) string {
	plotWidth := max(8, min(54, ctx.width-28))
	all := domain.AggregateDrawdown(domain.DrawdownSeries(ctx.data.Curve), ctx.grain)
	visible := domain.PointWindow(all, max(1, plotWidth/2), ctx.offset)
	heading := theme.Accent.Render("DRAWDOWN / HISTORY · " + strings.ToUpper(string(ctx.grain)))
	body := heading + "\n" +
		theme.Muted.Render("Worst daily decline from prior peak in each period (%)") + "\n\n" +
		chart.Drawdown(visible, plotWidth, 7) + "\n" +
		theme.Muted.Render("z interval · ←/→ move one period · 0% = previous peak")
	return ctx.style.Render(body)
}

func renderAllocationCard(ctx analysisRenderContext) string {
	p := ctx.data.Portfolio
	barWidth := max(12, min(48, ctx.width-26))
	var b strings.Builder
	b.WriteString(theme.Accent.Render("TARGET ALLOCATION") + "\n")
	for _, pos := range p.Positions {
		if pos.Allocation <= 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("%-10s %s %5.1f%%\n", pos.Symbol, chart.HorizontalBar(pos.Allocation, 1, barWidth, false), pos.Allocation*100))
	}
	b.WriteString(theme.Muted.Render("Normalized target weights; not drifting market weights."))
	return ctx.style.Render(b.String())
}

func renderPeriodReturnsCard(ctx analysisRenderContext) string {
	p := ctx.data.Portfolio
	curve := ctx.data.Curve
	capital := ctx.data.Capital
	if capital <= 0 {
		capital = 10000
	}
	returns := domain.PeriodReturns(curve, ctx.grain)
	gains := domain.PeriodGains(curve, ctx.grain, capital)
	gainByDate := make(map[string]float64, len(gains))
	for _, gain := range gains {
		gainByDate[gain.Date.Format("2006-01-02")] = gain.Value
	}
	plotWidth := max(8, min(54, ctx.width-28))
	returns = domain.PointWindow(returns, max(1, plotWidth/2), ctx.offset)
	var b strings.Builder
	b.WriteString(theme.Accent.Render("PERIOD RETURNS · "+strings.ToUpper(string(ctx.grain))) + "\n")
	if len(returns) > 0 {
		latest := returns[len(returns)-1]
		b.WriteString(fmt.Sprintf("Latest  %s  %s  %s\n\n",
			latest.Date.Format("2006-01-02"),
			theme.ColoredPercent(latest.Value),
			theme.ColoredAmount(gainByDate[latest.Date.Format("2006-01-02")], p.BaseCurrency),
		))
	}
	b.WriteString(chart.ReturnBars(returns, plotWidth, 7, capital, p.BaseCurrency))
	b.WriteString("\n" + theme.Gain.Render("━━/░░ Gain above 0%") + "   " + theme.Loss.Render("━━/░░ Loss below 0%"))
	b.WriteString("\n" + theme.Muted.Render("Y axis: "+p.BaseCurrency+" model gain/loss · bar heights are percentage returns"))
	if len(returns) > 0 {
		best, worst := returns[0], returns[0]
		for _, point := range returns[1:] {
			if point.Value > best.Value {
				best = point
			}
			if point.Value < worst.Value {
				worst = point
			}
		}
		b.WriteString(fmt.Sprintf("\nBest %s   Worst %s", theme.ColoredPercent(best.Value), theme.ColoredPercent(worst.Value)))
	}
	b.WriteString("\n" + theme.Muted.Render("z interval · ←/→ previous/next periods · first period may be partial"))
	return ctx.style.Render(b.String())
}

func renderWeightDriftCard(ctx analysisRenderContext) string {
	p := ctx.data.Portfolio
	snapshots := ctx.data.Snapshots
	targets := ctx.data.Targets
	positions := domain.PositivePositions(p)
	heading := theme.Accent.Render("WEIGHT DRIFT · " + strings.ToUpper(string(ctx.grain)))
	if len(snapshots) == 0 || len(positions) != len(targets) {
		return ctx.style.Render(heading + "\n" + theme.Muted.Render("No weight history available."))
	}
	latest := snapshots[len(snapshots)-1]
	if len(latest.Weights) != len(targets) {
		return ctx.style.Render(heading + "\n" + theme.Muted.Render("No weight history available."))
	}
	barWidth := max(18, min(52, ctx.width-24))
	var b strings.Builder
	b.WriteString(heading + "\n")
	b.WriteString(theme.Muted.Render("LATEST ALLOCATION") + "\n")
	b.WriteString(theme.Muted.Render(fmt.Sprintf("%-8s", "TARGET")) + chart.StackedWeights(targets, barWidth) + "\n")
	b.WriteString(theme.Muted.Render(fmt.Sprintf("%-8s", "ACTUAL")) + chart.StackedWeights(latest.Weights, barWidth) + "\n\n")

	type driftRow struct {
		index  int
		target float64
		actual float64
		drift  float64
	}
	rows := make([]driftRow, 0, len(positions))
	for i := range positions {
		target, actual := targets[i]*100, latest.Weights[i]*100
		rows = append(rows, driftRow{i, target, actual, actual - target})
	}
	sort.Slice(rows, func(i, j int) bool { return math.Abs(rows[i].drift) > math.Abs(rows[j].drift) })
	maxRowDrift := 0.0
	for _, row := range rows {
		maxRowDrift = max(maxRowDrift, math.Abs(row.drift))
	}
	driftBarWidth := max(4, min(14, ctx.width-55))
	b.WriteString(theme.Muted.Render("DRIFT BY ASSET · largest first") + "\n")
	for _, row := range rows {
		bar := chart.HorizontalBar(row.drift, maxRowDrift, driftBarWidth, true)
		swatch := lipgloss.NewStyle().Foreground(theme.DriftColor(row.index)).Render("■")
		b.WriteString(fmt.Sprintf("%s %-7s %5.1f%% → %5.1f%%  %s %s\n",
			swatch, positions[row.index].Symbol, row.target, row.actual, bar, theme.ColoredPP(row.drift)))
	}

	history := domain.PointWindow(domain.MaxDriftSeries(snapshots, targets, ctx.grain), max(8, min(24, ctx.width/3)), ctx.offset)
	if len(history) > 0 {
		maxPoint := history[len(history)-1]
		for _, point := range history {
			if point.Value > maxPoint.Value {
				maxPoint = point
			}
		}
		b.WriteString(fmt.Sprintf("\nMax drift  %s · peak %s\n", theme.ColoredPP(maxPoint.Value), maxPoint.Date.Format("2006-01-02")))
		b.WriteString(theme.Muted.Render("MAX ABSOLUTE DRIFT OVER TIME") + "\n")
		b.WriteString(chart.DriftHistory(history, max(20, min(68, ctx.width-16)), 4))
	}
	b.WriteString("\n" + theme.Muted.Render("z interval · ←/→ drift history · target weights normalized to 100%"))
	return ctx.style.Render(strings.TrimRight(b.String(), "\n"))
}

func renderContributionCard(ctx analysisRenderContext) string {
	p := ctx.data.Portfolio
	heading := theme.Accent.Render("ASSET CONTRIBUTION · " + strings.ToUpper(string(ctx.grain)))
	periods := domain.ContributionHistory(p, ctx.data.Snapshots, ctx.data.Targets, ctx.grain)
	if len(periods) == 0 {
		return ctx.style.Render(heading + "\n" + theme.Muted.Render("No contribution history available."))
	}
	periodIndex := max(0, min(len(periods)-1-ctx.offset, len(periods)-1))
	period := periods[periodIndex]
	positions := domain.PositivePositions(p)
	capital := ctx.data.Capital
	if capital <= 0 {
		capital = 10000
	}
	barWidth := max(6, min(18, ctx.width-42))
	scale := 0.0
	for _, value := range period.Assets {
		scale = max(scale, math.Abs(value))
	}
	var b strings.Builder
	b.WriteString(heading + "\n" + theme.Muted.Render("Period ending "+period.Date.Format("2006-01-02")) + "\n")
	for i, pos := range positions {
		if i >= len(period.Assets) {
			break
		}
		value := period.Assets[i]
		bar := chart.HorizontalBar(value, scale, barWidth, false)
		amount := capital * value / 100
		b.WriteString(fmt.Sprintf("%-7s %s %s  %s\n", pos.Symbol, bar, theme.ColoredPercent(value), theme.ColoredAmount(amount, p.BaseCurrency)))
	}
	b.WriteString(fmt.Sprintf("\nTotal      %s  %s\n", theme.ColoredPercent(period.Total), theme.ColoredAmount(capital*period.Total/100, p.BaseCurrency)))
	b.WriteString(theme.Muted.Render("Contribution to model return · rebalance days use target weights\nz interval · ←/→ previous periods"))
	return ctx.style.Render(strings.TrimRight(b.String(), "\n"))
}
