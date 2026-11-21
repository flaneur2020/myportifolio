package ui

import (
	"fmt"
	"math"
	"strings"

	"portifolio/internal/plugin"
	"portifolio/internal/ui/chart"
	"portifolio/internal/ui/theme"
)

type pluginCardAdapter struct {
	card *plugin.Card
}

func (a pluginCardAdapter) ID() string             { return a.card.ID }
func (a pluginCardAdapter) Title() string          { return a.card.Title }
func (a pluginCardAdapter) SupportsInterval() bool { return a.card.SupportsInterval }
func (a pluginCardAdapter) SupportsPan() bool      { return a.card.SupportsPan }

func (a pluginCardAdapter) Render(ctx analysisRenderContext) string {
	blocks, err := a.card.Evaluate(plugin.Input{
		Portfolio: ctx.data.Portfolio,
		Curve:     ctx.data.Curve,
		Snapshots: ctx.data.Snapshots,
		Bars:      ctx.bars,
		Grain:     ctx.grain,
		Offset:    ctx.offset,
		Width:     ctx.width,
	})
	if err != nil {
		return ctx.style.Render(theme.Accent.Render(a.card.Title) + "\n" + theme.Muted.Render("Plugin error: "+err.Error()))
	}
	body := renderPluginBlocks(blocks, ctx.width)
	return ctx.style.Render(theme.Accent.Render(a.card.Title) + "\n" + body)
}

func renderPluginBlocks(blocks []plugin.Block, width int) string {
	var out strings.Builder
	for i, block := range blocks {
		if i > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(renderPluginBlock(block, width))
	}
	return strings.TrimRight(out.String(), "\n")
}

func renderPluginBlock(block plugin.Block, width int) string {
	switch block.Kind {
	case plugin.BlockText:
		return block.Text
	case plugin.BlockMetrics:
		if len(block.MetricRows) == 0 {
			return theme.Muted.Render("No metrics")
		}
		maxLabel := 0
		for _, row := range block.MetricRows {
			maxLabel = max(maxLabel, len([]rune(row[0])))
		}
		return theme.RenderMetricRows(block.MetricRows, maxLabel)
	case plugin.BlockTable:
		return renderPluginTable(block.Headers, block.TableRows)
	case plugin.BlockLine:
		if len(block.Points) < 2 {
			return theme.Muted.Render("No data")
		}
		body := chart.Line(block.Points, max(20, width-16), block.Height)
		if block.Title != "" {
			return block.Title + "\n" + body
		}
		return body
	case plugin.BlockBars:
		return renderPluginBars(block.BarRows, width)
	default:
		return theme.Muted.Render("[unsupported plugin block: " + block.RawKind + "]")
	}
}

func renderPluginTable(headers []string, rows [][]string) string {
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

func renderPluginBars(rows []plugin.BarRow, width int) string {
	if len(rows) == 0 {
		return theme.Muted.Render("No bars")
	}
	scale := 0.0
	for _, row := range rows {
		scale = max(scale, math.Abs(row.Value))
	}
	barWidth := max(6, min(24, width-38))
	var b strings.Builder
	for _, row := range rows {
		bar := chart.HorizontalBar(row.Value, scale, barWidth, false)
		b.WriteString(fmt.Sprintf("%-9s %s %+.2f%s\n", row.Label, bar, row.Value, row.Suffix))
	}
	return strings.TrimRight(b.String(), "\n")
}
