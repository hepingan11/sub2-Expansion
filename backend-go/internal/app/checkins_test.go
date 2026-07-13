package app

import "testing"

func TestBuildSocialBindingURL(t *testing.T) {
	got := buildSocialBindingURL("https://example.com/", "telegram", "user 123", "ABCDEFGH")
	want := "https://example.com/?invitecode=ABCDEFGH&platform=telegram&userid=user+123"
	if got != want {
		t.Fatalf("buildSocialBindingURL() = %q, want %q", got, want)
	}
}

func TestBuildSocialBindingURLWithoutBase(t *testing.T) {
	got := buildSocialBindingURL("", "wechat", "abc", "")
	want := "/?platform=wechat&userid=abc"
	if got != want {
		t.Fatalf("buildSocialBindingURL() = %q, want %q", got, want)
	}
}

func TestFirstHeaderValue(t *testing.T) {
	got := firstHeaderValue("https, http")
	want := "https"
	if got != want {
		t.Fatalf("firstHeaderValue() = %q, want %q", got, want)
	}
}

func TestIsValidPublicHost(t *testing.T) {
	valid := []string{"example.com", "example.com:6779", "127.0.0.1:5173"}
	for _, host := range valid {
		if !isValidPublicHost(host) {
			t.Fatalf("isValidPublicHost(%q) = false, want true", host)
		}
	}

	invalid := []string{"", "example.com/path", "example.com\nbad", "example.com bad"}
	for _, host := range invalid {
		if isValidPublicHost(host) {
			t.Fatalf("isValidPublicHost(%q) = true, want false", host)
		}
	}
}
