package domain

import (
	"fmt"
	"time"
)

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
	case "monthly":
		return GrainMonth, true
	case "quarterly":
		return GrainQuarter, true
	case "yearly":
		return GrainYear, true
	default:
		return "", false
	}
}
