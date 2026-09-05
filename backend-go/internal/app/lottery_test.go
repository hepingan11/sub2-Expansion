package app

import (
	"bytes"
	"testing"
	"time"
)

func TestParseLotteryPeriod(t *testing.T) {
	end := time.Now().Add(-time.Minute).UTC()
	start := end.AddDate(0, -1, 0)
	parsedStart, parsedEnd, minimum, err := parseLotteryPeriod(start.Format(time.RFC3339), end.Format(time.RFC3339), "5")
	if err != nil {
		t.Fatalf("parseLotteryPeriod() error = %v", err)
	}
	if !parsedStart.Equal(start.Truncate(time.Second)) || !parsedEnd.Equal(end.Truncate(time.Second)) {
		t.Fatal("parsed period does not match")
	}
	if minimum.StringFixed(2) != "5.00" {
		t.Fatalf("minimum = %s", minimum.StringFixed(2))
	}
}

func TestParseLotteryPeriodRejectsInvalidRules(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name, start, end, minimum string
	}{
		{"reversed", now.Format(time.RFC3339), now.Add(-time.Hour).Format(time.RFC3339), "5"},
		{"zero minimum", now.Add(-time.Hour).Format(time.RFC3339), now.Format(time.RFC3339), "0"},
		{"too long", now.Add(-367 * 24 * time.Hour).Format(time.RFC3339), now.Format(time.RFC3339), "5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, _, err := parseLotteryPeriod(test.start, test.end, test.minimum); err == nil {
				t.Fatal("parseLotteryPeriod() unexpectedly succeeded")
			}
		})
	}
}

func TestSelectRandomCandidateIndexesUnique(t *testing.T) {
	indexes, err := selectRandomCandidateIndexes(5, 3, bytes.NewReader(make([]byte, 64)))
	if err != nil {
		t.Fatalf("selectRandomCandidateIndexes() error = %v", err)
	}
	seen := map[int]bool{}
	for _, index := range indexes {
		if index < 0 || index >= 5 || seen[index] {
			t.Fatalf("invalid indexes: %v", indexes)
		}
		seen[index] = true
	}
}
