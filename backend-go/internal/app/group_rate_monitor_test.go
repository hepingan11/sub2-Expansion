package app

import "testing"

func TestAppendGroupRatePointMovesUnchangedRateToLatestTime(t *testing.T) {
	series := Sub2APIGroupRateSeries{Points: []Sub2APIGroupRatePoint{
		{Time: "2026-07-08 00:00:00", Rate: 0.1},
	}}

	appendGroupRatePoint(&series, Sub2APIGroupRatePoint{
		Time: "2026-07-09 00:00:00",
		Rate: 0.1,
	})

	if len(series.Points) != 1 {
		t.Fatalf("expected one point, got %d", len(series.Points))
	}
	if series.Points[0].Time != "2026-07-09 00:00:00" {
		t.Fatalf("expected latest point time, got %s", series.Points[0].Time)
	}
}

func TestAppendGroupRatePointKeepsRateChanges(t *testing.T) {
	series := Sub2APIGroupRateSeries{Points: []Sub2APIGroupRatePoint{
		{Time: "2026-07-08 00:00:00", Rate: 0.1},
	}}

	appendGroupRatePoint(&series, Sub2APIGroupRatePoint{
		Time: "2026-07-09 00:00:00",
		Rate: 0.2,
	})

	if len(series.Points) != 2 {
		t.Fatalf("expected two points, got %d", len(series.Points))
	}
	if series.Points[1].Rate != 0.2 {
		t.Fatalf("expected changed rate 0.2, got %v", series.Points[1].Rate)
	}
}

func TestAppendGroupRatePointIgnoresOlderSnapshot(t *testing.T) {
	series := Sub2APIGroupRateSeries{Points: []Sub2APIGroupRatePoint{
		{Time: "2026-07-09 00:00:00", Rate: 0.2},
	}}

	appendGroupRatePoint(&series, Sub2APIGroupRatePoint{
		Time: "2026-07-08 00:00:00",
		Rate: 0.1,
	})

	if len(series.Points) != 1 || series.Points[0].Rate != 0.2 {
		t.Fatalf("expected newer point to remain unchanged, got %+v", series.Points)
	}
}
