package ui

import (
	"strings"
)

type security struct{ Symbol, Name, Market, Kind string }

// Match ordered subsequences; prefer exact symbols, prefixes, and compact matches.
func fuzzyScore(query, text string) (int, bool) {
	if text == query || strings.Split(text, " · ")[0] == query {
		return -1000, true
	}
	if strings.HasPrefix(text, query) {
		return -500, true
	}
	if i := strings.Index(text, query); i >= 0 {
		return i, true
	}
	q := []rune(query)
	at, first, last := 0, -1, 0
	for i, r := range []rune(text) {
		if at < len(q) && r == q[at] {
			if first < 0 {
				first = i
			}
			last = i
			at++
		}
	}
	if at != len(q) {
		return 0, false
	}
	return 100 + first + (last - first - len(q)), true
}
