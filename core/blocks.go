package core

import (
	"regexp"
	"strings"
	"unicode"
)

// Tone represents semantic text styling without allowing raw ANSI escape codes.
type Tone string

const (
	ToneDefault   Tone = ""
	ToneMuted     Tone = "muted"
	ToneAccent    Tone = "accent"
	ToneHighlight Tone = "highlight"
	ToneGain      Tone = "gain"
	ToneLoss      Tone = "loss"
	ToneNeutral   Tone = "neutral"
)

// TextSpan is an inline styled text segment inside a line or metric value.
type TextSpan struct {
	Text string
	Tone Tone
}

type BlockKind string

const (
	BlockText         BlockKind = "text"
	BlockSpans        BlockKind = "spans"
	BlockMetrics      BlockKind = "metrics"
	BlockTable        BlockKind = "table"
	BlockLine         BlockKind = "line"
	BlockBars         BlockKind = "bars"
	BlockCandles      BlockKind = "candles"
	BlockDrawdown     BlockKind = "drawdown"
	BlockReturnBars   BlockKind = "return_bars"
	BlockStackedBar   BlockKind = "stacked_bar"
	BlockDriftHistory BlockKind = "drift_history"
	BlockSpacer       BlockKind = "spacer"
	BlockUnsupported  BlockKind = "unsupported"
)

type MetricRow struct {
	Label     string
	Value     string
	Tone      Tone
	Spans     []TextSpan
	Separator bool
}

type BarRow struct {
	Label       string
	Value       float64
	Suffix      string
	Precision   int  // defaults to 2 if <= 0 and !PrecisionSet
	PrecisionSet bool
	HideSign    bool
	SwatchIndex int // -1 means no swatch
	Prefix      string
	ExtraSpans  []TextSpan
}

type StackedRow struct {
	Label   string
	Weights []float64
}

// Block is the UI-agnostic declarative intermediate representation returned by
// both built-in and user analysis card plugins.
type Block struct {
	Kind         BlockKind
	RawKind      string
	Text         string
	Tone         Tone
	Spans        []TextSpan
	Title        string
	Height       int
	LabelWidth   int
	MetricRows   []MetricRow
	Headers      []string
	TableRows    [][]string
	Points       []Point
	Bars         []Bar
	Grain        Grain
	Capital      float64
	Currency     string
	BarRows      []BarRow
	BarScale     float64
	BarMinOne    bool
	BarWidthCap  int
	BarLabelWidth int
	StackedRows  []StackedRow
}

// CardResult is returned by Card.Evaluate.
type CardResult struct {
	TitleSuffix string
	Blocks      []Block
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

func SanitizeInline(text string) string {
	text = SanitizeText(text)
	return strings.ReplaceAll(text, "\n", " ")
}
