package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"portifolio/core"
	"portifolio/ui/chart"
	"portifolio/ui/theme"
)

func renderCard(cache *cardEvalCache, card core.Card, in core.CardInput, style lipgloss.Style) string {
	meta := card.Meta()
	res, err := cache.evaluate(card, in)
	if err != nil {
		return style.Render(theme.Accent.Render(meta.Title) + "\n" + theme.Muted.Render("Plugin error: "+err.Error()))
	}
	heading := meta.Title
	if res.TitleSuffix != "" {
		heading += res.TitleSuffix
	}
	body := renderBlocks(res.Blocks, in.Width)
	return style.Render(theme.Accent.Render(heading) + "\n" + body)
}

func renderBlocks(blocks []core.Block, width int) string {
	var out strings.Builder
	for i, block := range blocks {
		if i > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(renderBlock(block, width))
	}
	return strings.TrimRight(out.String(), "\n")
}

func renderBlock(block core.Block, width int) string {
	switch block.Kind {
	case core.BlockSpacer:
		return ""
	case core.BlockText:
		return theme.RenderTone(block.Tone, block.Text)
	case core.BlockSpans:
		return theme.RenderSpans(block.Spans)
	case core.BlockMetrics:
		return renderMetricBlock(block)
	case core.BlockTable:
		return renderTableBlock(block.Headers, block.TableRows)
	case core.BlockLine:
		if len(block.Points) < 2 {
			return theme.Muted.Render("No data")
		}
		body := chart.Line(block.Points, max(20, width-16), block.Height)
		if block.Title != "" {
			return block.Title + "\n" + body
		}
		return body
	case core.BlockCandles:
		chartWidth := max(20, min(68, width-16))
		grain := block.Grain
		if grain == "" {
			grain = core.GrainDay
		}
		body := chart.Candles(block.Bars, grain, chartWidth, block.Height)
		if block.Title != "" {
			return block.Title + "\n" + body
		}
		return body
	case core.BlockDrawdown:
		plotWidth := max(8, min(54, width-28))
		body := chart.Drawdown(block.Points, plotWidth, block.Height)
		if block.Title != "" {
			return block.Title + "\n" + body
		}
		return body
	case core.BlockReturnBars:
		plotWidth := max(8, min(54, width-28))
		capital := block.Capital
		if capital <= 0 {
			capital = 10000
		}
		return chart.ReturnBars(block.Points, plotWidth, block.Height, capital, block.Currency)
	case core.BlockStackedBar:
		return renderStackedBlock(block.StackedRows, width)
	case core.BlockDriftHistory:
		body := chart.DriftHistory(block.Points, max(20, min(68, width-16)), block.Height)
		if block.Title != "" {
			return block.Title + "\n" + body
		}
		return body
	case core.BlockBars:
		return renderBarsBlock(block, width)
	default:
		return theme.Muted.Render("[unsupported plugin block: " + block.RawKind + "]")
	}
}

func renderMetricBlock(block core.Block) string {
	if len(block.MetricRows) == 0 {
		return theme.Muted.Render("No metrics")
	}
	labelWidth := block.LabelWidth
	if labelWidth <= 0 {
		for _, row := range block.MetricRows {
			labelWidth = max(labelWidth, len([]rune(row.Label)))
		}
	}
	rows := make([][2]string, 0, len(block.MetricRows))
	for _, row := range block.MetricRows {
		if row.Separator {
			rows = append(rows, [2]string{})
			continue
		}
		val := theme.RenderTone(row.Tone, row.Value)
		if len(row.Spans) > 0 {
			val = theme.RenderSpans(row.Spans)
		}
		rows = append(rows, [2]string{row.Label, val})
	}
	return theme.RenderMetricRows(rows, labelWidth)
}

func renderStackedBlock(rows []core.StackedRow, width int) string {
	barWidth := max(18, min(52, width-24))
	var b strings.Builder
	for i, row := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(theme.Muted.Render(fmt.Sprintf("%-8s", row.Label)) + chart.StackedWeights(row.Weights, barWidth))
	}
	return b.String()
}

func renderTableBlock(headers []string, rows [][]string) string {
	if len(rows) == 0 && len(headers) == 0 {
		return theme.Muted.Render("No table rows")
	}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len([]rune(h))
	}
	for _, row := range rows {
		for col, val := range row {
			for col >= len(widths) {
				widths = append(widths, 0)
			}
			widths[col] = max(widths[col], len([]rune(val)))
		}
	}
	var b strings.Builder
	if len(headers) > 0 {
		for i, h := range headers {
			b.WriteString(theme.Muted.Render(fmt.Sprintf("%-*s", widths[i], h)))
			if i < len(headers)-1 {
				b.WriteString("  ")
			}
		}
		b.WriteByte('\n')
	}
	for _, row := range rows {
		for i, val := range row {
			b.WriteString(fmt.Sprintf("%-*s", widths[i], val))
			if i < len(row)-1 {
				b.WriteString("  ")
			}
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderBarsBlock(block core.Block, width int) string {
	rows := block.BarRows
	if len(rows) == 0 {
		return theme.Muted.Render("No bars")
	}
	scale := block.BarScale
	if scale <= 0 {
		for _, row := range rows {
			scale = max(scale, math.Abs(row.Value))
		}
	}
	labelWidth := block.BarLabelWidth
	if labelWidth <= 0 {
		labelWidth = 9
	}
	barWidth := max(6, min(24, width-38))
	switch {
	case block.BarWidthCap >= 40:
		barWidth = max(12, min(block.BarWidthCap, width-26))
	case block.BarWidthCap > 0 && block.BarWidthCap <= 14:
		barWidth = max(4, min(block.BarWidthCap, width-55))
	case block.BarWidthCap > 0:
		barWidth = max(6, min(block.BarWidthCap, width-42))
	}

	var b strings.Builder
	for _, row := range rows {
		bar := chart.HorizontalBar(row.Value, scale, barWidth, block.BarMinOne)
		if row.SwatchIndex >= 0 {
			swatch := lipgloss.NewStyle().Foreground(theme.DriftColor(row.SwatchIndex)).Render("■")
			b.WriteString(swatch + " ")
		}
		b.WriteString(fmt.Sprintf("%-*s ", labelWidth, row.Label))
		if row.Prefix != "" {
			b.WriteString(row.Prefix + " ")
		}
		b.WriteString(bar + " ")
		if len(row.ExtraSpans) > 0 {
			b.WriteString(theme.RenderSpans(row.ExtraSpans))
		} else {
			prec := 2
			if row.PrecisionSet {
				prec = max(0, row.Precision)
			}
			if row.HideSign {
				b.WriteString(fmt.Sprintf("%5.*f%s", prec, row.Value, row.Suffix))
			} else {
				b.WriteString(fmt.Sprintf("%+.*f%s", prec, row.Value, row.Suffix))
			}
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
