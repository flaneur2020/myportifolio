package chart

import (
	"math"
	"strings"
	"time"

	"portifolio/internal/ui/theme"
)

// DateAxis places sparse date labels aligned with two-column periods without overlap.
// When minWidth > 0, the axis uses full YYYY-MM-DD labels with at least minWidth columns.
// When latestFirst is true, the rightmost date is reserved before placing earlier dates.
func DateAxis(dates []time.Time, indent string, minWidth int, latestFirst bool) string {
	if len(dates) == 0 {
		return ""
	}
	columns := len(dates) * 2
	format := "2006-01-02"
	if minWidth == 0 && columns < 10 {
		format = "01-02"
	}
	labelWidth := len(dates[0].Format(format))
	axisLen := max(minWidth, columns)
	if axisLen < labelWidth {
		return ""
	}
	axis := make([]rune, axisLen)
	for i := range axis {
		axis[i] = ' '
	}
	if latestFirst {
		occupied := make([]bool, len(axis))
		place := func(i int) {
			label := dates[i].Format(format)
			pos := i * 2
			if i == len(dates)-1 {
				pos = max(0, len(axis)-len(label))
			}
			if pos+len(label) > len(axis) {
				return
			}
			for j := pos; j < pos+len(label); j++ {
				if occupied[j] {
					return
				}
			}
			copy(axis[pos:], []rune(label))
			for j := pos; j < pos+len(label); j++ {
				occupied[j] = true
			}
		}
		place(len(dates) - 1)
		for i := 0; i < len(dates)-1; i++ {
			place(i)
		}
	} else {
		next := 0
		for i, d := range dates {
			label := d.Format(format)
			pos := i * 2
			if i == len(dates)-1 {
				pos = max(0, len(axis)-len(label))
			}
			if pos < next || pos+len(label) > len(axis) {
				continue
			}
			copy(axis[pos:], []rune(label))
			next = pos + len(label) + 2
		}
	}
	return theme.Muted.Render(indent + strings.TrimRight(string(axis), " "))
}

// HorizontalBar renders a colored proportional bar using "━" glyphs.
func HorizontalBar(value, scale float64, width int, minOne bool) string {
	n := 0
	if scale > 0 {
		n = int(math.Round(math.Abs(value) / scale * float64(width)))
		if minOne {
			n = max(1, n)
		}
	}
	bar := strings.Repeat("━", n)
	if value < 0 {
		return theme.Loss.Render(bar)
	}
	return theme.Accent.Render(bar)
}
