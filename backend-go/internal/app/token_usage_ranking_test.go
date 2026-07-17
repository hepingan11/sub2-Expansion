package app

import "testing"

func TestNormalizeUserTokenUsageRanking(t *testing.T) {
	points := []sub2APIUserUsageTrendPoint{
		{UserID: 3, Username: "charlie", Requests: 2, Tokens: 300},
		{UserID: 1, Email: "alice@example.com", Requests: 4, Tokens: 500},
		{UserID: 2, Username: "bob", Requests: 5, Tokens: 500},
		{UserID: 3, Requests: 1, Tokens: 250},
		{UserID: 0, Username: "invalid", Requests: 99, Tokens: 9999},
		{UserID: 4, Username: "negative", Requests: -1, Tokens: -1},
	}

	ranking := normalizeUserTokenUsageRanking(points, 3, 3)
	if len(ranking) != 3 {
		t.Fatalf("len(ranking) = %d, want 3", len(ranking))
	}
	if ranking[0].Rank != 1 || ranking[0].Tokens != 550 || !ranking[0].IsCurrentUser {
		t.Fatalf("ranking[0] = %#v", ranking[0])
	}
	if ranking[1].Rank != 2 || ranking[1].DisplayName != "b***b" || ranking[1].Tokens != 500 {
		t.Fatalf("ranking[1] = %#v", ranking[1])
	}
	if ranking[2].Rank != 3 || ranking[2].DisplayName != "a***e@example.com" {
		t.Fatalf("ranking[2] = %#v", ranking[2])
	}
}

func TestMaskedRankingIdentityFallback(t *testing.T) {
	if got := maskedRankingIdentity("", "", 42); got != "用户 #42" {
		t.Fatalf("maskedRankingIdentity() = %q", got)
	}
	if got := maskedRankingIdentity("李雷", "", 42); got != "李***" {
		t.Fatalf("maskedRankingIdentity() = %q", got)
	}
}
