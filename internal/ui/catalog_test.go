package ui

import "testing"

func TestFuzzyMatching(t *testing.T) {
	for _, q := range []string{"vti", "vgd", "total"} {
		if _, ok := fuzzyScore(q, "vti vanguard total stock market"); !ok {
			t.Fatalf("failed query %q", q)
		}
	}
	if _, ok := fuzzyScore("zzzz", "vti vanguard"); ok {
		t.Fatal("unexpected match")
	}
}
