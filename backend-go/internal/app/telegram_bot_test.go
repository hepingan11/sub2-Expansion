package app

import "testing"

func TestParseTelegramCommand(t *testing.T) {
	command, arg := parseTelegramCommand("/checkin@my_bot now")
	if command != "checkin" {
		t.Fatalf("command = %q, want checkin", command)
	}
	if arg != "now" {
		t.Fatalf("arg = %q, want now", arg)
	}
}

func TestParseTelegramCommandIgnoresPlainText(t *testing.T) {
	command, arg := parseTelegramCommand("checkin")
	if command != "" || arg != "" {
		t.Fatalf("parseTelegramCommand() = %q, %q; want blanks", command, arg)
	}
}
