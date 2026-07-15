package app

import "testing"

func TestNormalizePlatformName(t *testing.T) {
	got, err := normalizePlatformName(" Telegram ")
	if err != nil {
		t.Fatalf("normalizePlatformName() error = %v", err)
	}
	if got != "telegram" {
		t.Fatalf("normalizePlatformName() = %q, want telegram", got)
	}

	if _, err := normalizePlatformName("bad platform"); err == nil {
		t.Fatal("normalizePlatformName() accepted a platform with spaces")
	}
}

func TestPlatformSettingKey(t *testing.T) {
	got := platformSettingKey("telegram", "check_in.prize_tiers")
	want := "platform.telegram.check_in.prize_tiers"
	if got != want {
		t.Fatalf("platformSettingKey() = %q, want %q", got, want)
	}
}

func TestPlatformMethodKey(t *testing.T) {
	got := platformMethodKey("telegram")
	want := "social:telegram"
	if got != want {
		t.Fatalf("platformMethodKey() = %q, want %q", got, want)
	}
}
