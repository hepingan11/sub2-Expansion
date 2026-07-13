package app

import (
	"testing"
	"time"
)

func TestNormalizeInvitationCode(t *testing.T) {
	got, err := normalizeInvitationCode(" abcdefgh ")
	if err != nil {
		t.Fatalf("normalizeInvitationCode() error = %v", err)
	}
	if got != "ABCDEFGH" {
		t.Fatalf("normalizeInvitationCode() = %q, want ABCDEFGH", got)
	}
	if _, err := normalizeInvitationCode("ABC0EFGH"); err == nil {
		t.Fatal("normalizeInvitationCode() accepted an excluded character")
	}
}

func TestNormalizeInvitationConfig(t *testing.T) {
	input := InvitationConfig{AfterTime: "2026-07-13T08:00:00+08:00", Amount: MustAmount("5.126")}
	got, err := normalizeInvitationConfig(input)
	if err != nil {
		t.Fatalf("normalizeInvitationConfig() error = %v", err)
	}
	if got.AfterTime != "2026-07-13T00:00:00Z" {
		t.Fatalf("AfterTime = %q", got.AfterTime)
	}
	if got.Amount.StringFixed(2) != "5.13" {
		t.Fatalf("Amount = %s", got.Amount.StringFixed(2))
	}
}

func TestInvitationCreatedAtMustBeStrictlyAfterThreshold(t *testing.T) {
	threshold, _ := time.Parse(time.RFC3339, "2026-07-13T00:00:00Z")
	cases := []struct {
		name      string
		createdAt time.Time
		eligible  bool
	}{
		{name: "before", createdAt: threshold.Add(-time.Second), eligible: false},
		{name: "equal", createdAt: threshold, eligible: false},
		{name: "after", createdAt: threshold.Add(time.Second), eligible: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.createdAt.After(threshold); got != tc.eligible {
				t.Fatalf("After() = %v, want %v", got, tc.eligible)
			}
		})
	}
}
