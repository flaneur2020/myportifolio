package ui

import (
	"fmt"
	"portifolio/internal/domain"
	"strings"
	"time"
)

func effectiveStartNote(requested string, effective time.Time, series []domain.AssetSeries) string {
	start, err := time.Parse("2006-01-02", requested)
	if err != nil || !effective.After(start) {
		return ""
	}
	var constraints []string
	for _, asset := range series {
		var first time.Time
		for _, point := range asset.Points {
			if point.Value > 0 && !point.Date.Before(start) && (first.IsZero() || point.Date.Before(first)) {
				first = point.Date
			}
		}
		if !first.IsZero() && first.After(start.AddDate(0, 0, 7)) {
			constraints = append(constraints, fmt.Sprintf("%s history begins %s", asset.Position.Symbol, first.Format("2006-01-02")))
		}
	}
	if len(constraints) == 0 {
		return ""
	}
	return muted.Render(fmt.Sprintf("Effective start %s · %s. Earlier dates require prices for every weighted asset.", effective.Format("2006-01-02"), strings.Join(constraints, "; "))) + "\n"
}
