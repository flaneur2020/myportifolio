package core

import (
	"fmt"
	"strings"
	"time"
)

const (
	RebalanceNever     = "never"
	RebalanceMonthly   = "monthly"
	RebalanceQuarterly = "quarterly"
	RebalanceYearly    = "yearly"
)

var RebalanceOptions = []string{
	RebalanceNever,
	RebalanceMonthly,
	RebalanceQuarterly,
	RebalanceYearly,
}

type Bar struct {
	Date                                  time.Time
	Open, High, Low, Close, AdjustedClose float64
	Currency                              string
	Source                                string
}

type Asset struct {
	Symbol, Name, Market, Kind string
}

type Point struct {
	Date  time.Time
	Value float64
}

type Grain string

const (
	GrainDay     Grain = "day"
	GrainWeek    Grain = "week"
	GrainMonth   Grain = "month"
	GrainQuarter Grain = "quarter"
	GrainYear    Grain = "year"
)

var Grains = []Grain{GrainDay, GrainWeek, GrainMonth, GrainQuarter, GrainYear}

type Config struct {
	Market    string
	Portfolio Portfolio
	Analysis  AnalysisConfig
	Theme     string
}

type AnalysisConfig struct {
	Order  []string
	Hidden []string
}

type Portfolio struct {
	ID             string
	Name           string
	BaseCurrency   string
	Market         string
	PriceCurrency  string
	InitialCapital float64
	StartDate      string
	Rebalance      string
	Positions      []Position
}

type Position struct {
	Symbol     string
	Market     string
	Kind       string
	Allocation float64
	Currency   string
}

// MarketSpec describes a market registered by a plugin (e.g. "us", "cn").
type MarketSpec struct {
	ID                string
	Name              string
	Description       string
	Currency          string
	AssetKinds        []string
	PriceProviders    []string
	DirectoryProvider string
	CommonSymbols     []string
}

func (m MarketSpec) Clone() MarketSpec {
	out := m
	out.AssetKinds = append([]string(nil), m.AssetKinds...)
	out.PriceProviders = append([]string(nil), m.PriceProviders...)
	out.CommonSymbols = append([]string(nil), m.CommonSymbols...)
	return out
}

func (m MarketSpec) AllowsKind(kind string) bool {
	if kind == "" {
		return false
	}
	if len(m.AssetKinds) == 0 {
		return true
	}
	for _, k := range m.AssetKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// ThemeSpec defines semantic UI color tokens provided by a theme plugin.
type ThemeSpec struct {
	ID           string
	Name         string
	Accent       string
	Muted        string
	Highlight    string
	Gain         string
	GainFill     string
	Loss         string
	LossFill     string
	Neutral      string
	BorderNormal string
	DriftBar     string
	Series       []string
}

func (t ThemeSpec) Clone() ThemeSpec {
	out := t
	out.Series = append([]string(nil), t.Series...)
	return out
}

func (c Config) Clone() Config {
	out := c
	out.Portfolio.Positions = append([]Position(nil), c.Portfolio.Positions...)
	out.Analysis.Order = append([]string(nil), c.Analysis.Order...)
	out.Analysis.Hidden = append([]string(nil), c.Analysis.Hidden...)
	return out
}

// StepGrain moves finer (delta < 0) or coarser (delta > 0) without wrapping.
func StepGrain(current Grain, delta int) Grain {
	index := 0
	for i, g := range Grains {
		if g == current {
			index = i
			break
		}
	}
	return Grains[max(0, min(len(Grains)-1, index+delta))]
}

// CycleGrain steps through Grains with wrap-around.
func CycleGrain(current Grain, delta int) Grain {
	index := 0
	for i, g := range Grains {
		if g == current {
			index = i
			break
		}
	}
	n := len(Grains)
	return Grains[((index+delta)%n+n)%n]
}

// RangeStart returns the recommended historical lookback start for an asset chart.
func RangeStart(end time.Time, grain Grain) time.Time {
	switch grain {
	case GrainWeek:
		return end.AddDate(-1, 0, 0)
	case GrainMonth:
		return end.AddDate(-5, 0, 0)
	case GrainQuarter:
		return end.AddDate(-15, 0, 0)
	case GrainYear:
		return end.AddDate(-40, 0, 0)
	default:
		return end.AddDate(0, -3, 0)
	}
}

// PeriodStart returns the canonical calendar start date for date in the given grain.
func PeriodStart(date time.Time, grain Grain) time.Time {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	switch grain {
	case GrainWeek:
		return date.AddDate(0, 0, -(int(date.Weekday())+6)%7)
	case GrainMonth:
		return time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	case GrainQuarter:
		return time.Date(date.Year(), time.Month((int(date.Month())-1)/3*3+1), 1, 0, 0, 0, 0, date.Location())
	case GrainYear:
		return time.Date(date.Year(), 1, 1, 0, 0, 0, 0, date.Location())
	default:
		return date
	}
}

// PeriodKey returns a lexicographically sortable bucket key for date in the given grain.
func PeriodKey(date time.Time, grain Grain) string {
	switch grain {
	case GrainWeek:
		y, w := date.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w)
	case GrainMonth:
		return date.Format("2006-01")
	case GrainQuarter:
		return fmt.Sprintf("%04d-Q%d", date.Year(), (int(date.Month())-1)/3+1)
	case GrainYear:
		return date.Format("2006")
	default:
		return date.Format("2006-01-02")
	}
}

func rebalanceGrain(rule string) (Grain, bool) {
	switch rule {
	case RebalanceMonthly:
		return GrainMonth, true
	case RebalanceQuarterly:
		return GrainQuarter, true
	case RebalanceYearly:
		return GrainYear, true
	default:
		return "", false
	}
}

func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// ValidID checks whether id uses only ASCII letters, digits, '-' or '_'.
func ValidID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
