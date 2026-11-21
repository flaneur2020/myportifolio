package plugin

import (
	"regexp"
	"strings"
	"unicode"

	"portifolio/internal/domain"
)

type BlockKind string

const (
	BlockText        BlockKind = "text"
	BlockMetrics     BlockKind = "metrics"
	BlockTable       BlockKind = "table"
	BlockLine        BlockKind = "line"
	BlockBars        BlockKind = "bars"
	BlockUnsupported BlockKind = "unsupported"
)

type BarRow struct {
	Label  string
	Value  float64
	Suffix string
}

// Block is the UI-agnostic intermediate representation returned by Lua plugins.
type Block struct {
	Kind       BlockKind
	RawKind    string
	Text       string
	Title      string
	Height     int
	MetricRows [][2]string
	Headers    []string
	TableRows  [][]string
	Points     []domain.Point
	BarRows    []BarRow
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func SanitizeText(text string) string {
	text = ansiPattern.ReplaceAllString(text, "")
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 32 || r == 127 || r == 0x1b {
			return -1
		}
		if !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, text)
}

func SanitizeLine(text string) string {
	return strings.Join(strings.Fields(SanitizeText(text)), " ")
}
