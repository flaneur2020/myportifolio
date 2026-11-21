package theme

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"portifolio/core"
)

var (
	ColorAccent       = "#55d6be"
	ColorMuted        = "#8995a5"
	ColorHighlight    = "#ffd166"
	ColorGain         = "#35c98b"
	ColorGainFill     = "#2f8f68"
	ColorLoss         = "#ff6375"
	ColorLossFill     = "#733b4b"
	ColorNeutral      = "#929ba5"
	ColorBorderNormal = "#3d4d59"
	ColorDriftBar     = "#ff9e64"

	Accent    lipgloss.Style
	Muted     lipgloss.Style
	Highlight lipgloss.Style

	Gain     lipgloss.Style
	GainFill lipgloss.Style
	Loss     lipgloss.Style
	LossFill lipgloss.Style
	Neutral  lipgloss.Style
	DriftBar lipgloss.Style

	DriftPalette = []string{"#55d6be", "#f2c14e", "#ff7b72", "#7aa2f7", "#bb9af7", "#9ece6a", "#ff9e64", "#7dcfff"}
)

func init() {
	rebuild()
}

func rebuild() {
	Accent = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	Muted = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))
	Highlight = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorHighlight))

	Gain = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGain))
	GainFill = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGainFill))
	Loss = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorLoss))
	LossFill = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorLossFill))
	Neutral = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorNeutral))
	DriftBar = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDriftBar))
}

// Apply updates semantic theme tokens from a registered core.ThemeSpec plugin.
func Apply(spec core.ThemeSpec) {
	if spec.Accent != "" {
		ColorAccent = spec.Accent
	}
	if spec.Muted != "" {
		ColorMuted = spec.Muted
	}
	if spec.Highlight != "" {
		ColorHighlight = spec.Highlight
	}
	if spec.Gain != "" {
		ColorGain = spec.Gain
	}
	if spec.GainFill != "" {
		ColorGainFill = spec.GainFill
	}
	if spec.Loss != "" {
		ColorLoss = spec.Loss
	}
	if spec.LossFill != "" {
		ColorLossFill = spec.LossFill
	}
	if spec.Neutral != "" {
		ColorNeutral = spec.Neutral
	}
	if spec.BorderNormal != "" {
		ColorBorderNormal = spec.BorderNormal
	}
	if spec.DriftBar != "" {
		ColorDriftBar = spec.DriftBar
	}
	if len(spec.Series) > 0 {
		DriftPalette = append([]string(nil), spec.Series...)
	}
	rebuild()
}

func RenderTone(tone core.Tone, text string) string {
	switch tone {
	case core.ToneMuted:
		return Muted.Render(text)
	case core.ToneAccent:
		return Accent.Render(text)
	case core.ToneHighlight:
		return Highlight.Render(text)
	case core.ToneGain:
		return Gain.Render(text)
	case core.ToneLoss:
		return Loss.Render(text)
	case core.ToneNeutral:
		return Neutral.Render(text)
	default:
		return text
	}
}

func RenderSpans(spans []core.TextSpan) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(RenderTone(s.Tone, s.Text))
	}
	return b.String()
}

func DriftColor(index int) lipgloss.Color {
	if len(DriftPalette) == 0 {
		return lipgloss.Color(ColorAccent)
	}
	if index < 0 {
		index = 0
	}
	return lipgloss.Color(DriftPalette[index%len(DriftPalette)])
}

func CardStyle(width int, focused bool) lipgloss.Style {
	color := ColorBorderNormal
	if focused {
		color = ColorHighlight
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(color)).
		Padding(0, 2).
		Width(max(18, min(76, width-8)))
}

// AnalysisCardStyle distinguishes selected (yellow rounded) vs active (cyan double).
func AnalysisCardStyle(width int, selected, active bool) lipgloss.Style {
	if !selected {
		return CardStyle(width, false)
	}
	if active {
		return lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color(ColorAccent)).
			Bold(true).
			Padding(0, 2).
			Width(max(18, min(76, width-8)))
	}
	return CardStyle(width, true)
}

func signStyle(value float64) lipgloss.Style {
	if value > 0 {
		return Gain
	}
	if value < 0 {
		return Loss
	}
	return Neutral
}

func ColoredAmount(value float64, currency string) string {
	return signStyle(value).Render(fmt.Sprintf("%+.2f %s", value, currency))
}

func ColoredPercent(value float64) string {
	return signStyle(value).Render(fmt.Sprintf("%+.2f%%", value))
}

func ColoredPP(value float64) string {
	return signStyle(value).Render(fmt.Sprintf("%+.1fpp", value))
}

func CurrencySymbol(currency string) string {
	switch strings.ToUpper(currency) {
	case "USD":
		return "$"
	case "CNY", "JPY":
		return "¥"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	default:
		return strings.ToUpper(currency) + " "
	}
}

func FormatCompactMoney(value float64, currency string) string {
	sign := "+"
	if value < 0 {
		sign = "-"
		value = -value
	} else if value == 0 {
		return CurrencySymbol(currency) + "0"
	}
	symbol := CurrencySymbol(currency)
	switch {
	case value >= 1e9:
		return fmt.Sprintf("%s%s%.1fB", sign, symbol, value/1e9)
	case value >= 1e6:
		return fmt.Sprintf("%s%s%.1fM", sign, symbol, value/1e6)
	case value >= 1e3:
		return fmt.Sprintf("%s%s%.1fK", sign, symbol, value/1e3)
	default:
		return fmt.Sprintf("%s%s%.0f", sign, symbol, value)
	}
}

// RenderMetricRows aligns values into a single column by padding plain labels
// before applying ANSI color codes.
func RenderMetricRows(rows [][2]string, labelWidth int) string {
	var b strings.Builder
	for _, row := range rows {
		if row[0] == "" {
			b.WriteByte('\n')
			continue
		}
		b.WriteString(Muted.Render(fmt.Sprintf("%-*s", labelWidth, row[0])))
		b.WriteString("  " + row[1] + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
