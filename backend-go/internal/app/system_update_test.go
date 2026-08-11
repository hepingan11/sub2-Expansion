package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func TestReadSystemUpdateStatusFromPathsReturnsIdleWithoutStateFile(t *testing.T) {
	tempDir := t.TempDir()
	got, err := readSystemUpdateStatusFromPaths(filepath.Join(tempDir, "state"), filepath.Join(tempDir, "log"), time.Now())
	if err != nil {
		t.Fatalf("readSystemUpdateStatusFromPaths() error = %v", err)
	}
	if got.Status != "IDLE" || got.Message != "暂无更新任务" {
		t.Fatalf("readSystemUpdateStatusFromPaths() = %#v, want idle status", got)
	}
}

func TestReadSystemUpdateStatusFromPathsMarksStaleTaskFailed(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state")
	logPath := filepath.Join(tempDir, "log")
	startedAt := time.Date(2026, 7, 27, 7, 0, 0, 0, time.UTC)
	state := strings.Join([]string{
		"task_id=update-1",
		"status=RUNNING",
		"current_version=v0.2.1",
		"target_version=v0.2.2",
		"started_at=" + startedAt.Format(time.RFC3339),
		"finished_at=",
		"message=Pulling release images.",
		"",
	}, "\n")
	if err := os.WriteFile(statePath, []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("pulling image\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := readSystemUpdateStatusFromPaths(statePath, logPath, startedAt.Add(systemUpdateMaxAge+time.Second))
	if err != nil {
		t.Fatalf("readSystemUpdateStatusFromPaths() error = %v", err)
	}
	if got.Status != "FAILED" || got.TaskID != "update-1" || got.TargetVersion != "v0.2.2" {
		t.Fatalf("readSystemUpdateStatusFromPaths() = %#v, want stale failed task", got)
	}
	if got.Output != "pulling image\n" {
		t.Fatalf("output = %q, want task log", got.Output)
	}
}

func TestReadFileTailKeepsLatestOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	if err := os.WriteFile(path, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readFileTail(path, 4)
	if err != nil {
		t.Fatalf("readFileTail() error = %v", err)
	}
	if got != "6789" {
		t.Fatalf("readFileTail() = %q, want %q", got, "6789")
	}
}
