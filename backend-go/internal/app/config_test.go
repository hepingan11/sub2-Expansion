package app

import "testing"

func TestLoadSystemUpdateCommandUsesConfiguredOverride(t *testing.T) {
	t.Setenv("SYSTEM_UPDATE_ENABLED", "true")
	t.Setenv("SYSTEM_UPDATE_COMMAND", "echo custom-update")
	if got := loadSystemUpdateCommand(); got != "echo custom-update" {
		t.Fatalf("loadSystemUpdateCommand() = %q, want configured command", got)
	}
}

func TestLoadSystemUpdateCommandCanBeDisabled(t *testing.T) {
	t.Setenv("SYSTEM_UPDATE_ENABLED", "false")
	t.Setenv("SYSTEM_UPDATE_COMMAND", "echo should-not-run")
	if got := loadSystemUpdateCommand(); got != "" {
		t.Fatalf("loadSystemUpdateCommand() = %q, want blank when disabled", got)
	}
}
