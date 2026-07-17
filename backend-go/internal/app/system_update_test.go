package app

import "testing"

func TestSemverPartsAcceptsTwoSegmentRelease(t *testing.T) {
	got, ok := semverParts("v0.2")
	if !ok {
		t.Fatal("semverParts() rejected v0.2")
	}
	want := [3]int{0, 2, 0}
	if got != want {
		t.Fatalf("semverParts() = %v, want %v", got, want)
	}
}

func TestReleaseIsNewerWithTwoSegmentRelease(t *testing.T) {
	if !releaseIsNewer("v0.1", "v0.2") {
		t.Fatal("releaseIsNewer() did not detect v0.2 as newer than v0.1")
	}
	if releaseIsNewer("v0.2", "v0.2.0") {
		t.Fatal("releaseIsNewer() treated v0.2.0 as newer than v0.2")
	}
}
