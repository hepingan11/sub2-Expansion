package app

import (
	"strings"
	"testing"
)

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

func TestTelegramMemberIsActive(t *testing.T) {
	tests := []struct {
		name   string
		member telegramChatMember
		want   bool
	}{
		{name: "creator", member: telegramChatMember{Status: "creator"}, want: true},
		{name: "administrator", member: telegramChatMember{Status: "administrator"}, want: true},
		{name: "member", member: telegramChatMember{Status: "member"}, want: true},
		{name: "restricted member", member: telegramChatMember{Status: "restricted", IsMember: true}, want: true},
		{name: "restricted non-member", member: telegramChatMember{Status: "restricted"}, want: false},
		{name: "left", member: telegramChatMember{Status: "left"}, want: false},
		{name: "kicked", member: telegramChatMember{Status: "kicked"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := telegramMemberIsActive(tt.member); got != tt.want {
				t.Fatalf("telegramMemberIsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTelegramBindingToken(t *testing.T) {
	app := &App{cfg: Config{AuthSecret: "test-secret"}}
	token, err := app.issueTelegramBindingToken("8576774398", "ABCDEFGH", 10)
	if err != nil {
		t.Fatalf("issueTelegramBindingToken() error = %v", err)
	}
	if err := app.verifyTelegramBindingToken(token, "8576774398", "ABCDEFGH"); err != nil {
		t.Fatalf("verifyTelegramBindingToken() error = %v", err)
	}
	if err := app.verifyTelegramBindingToken(token, "other-user", "ABCDEFGH"); err == nil {
		t.Fatal("verifyTelegramBindingToken() accepted a different Telegram user")
	}
	if err := app.verifyTelegramBindingToken(token, "8576774398", "BCDEFGHJ"); err == nil {
		t.Fatal("verifyTelegramBindingToken() accepted a different invitation code")
	}
	parts := strings.Split(token, ".")
	tampered := parts[0] + ".invalid-signature"
	if err := app.verifyTelegramBindingToken(tampered, "8576774398", "ABCDEFGH"); err == nil {
		t.Fatal("verifyTelegramBindingToken() accepted a tampered signature")
	}
}

func TestTelegramBindingTokenExpires(t *testing.T) {
	app := &App{cfg: Config{AuthSecret: "test-secret"}}
	token, err := app.issueTelegramBindingToken("8576774398", "", -1)
	if err != nil {
		t.Fatalf("issueTelegramBindingToken() error = %v", err)
	}
	if err := app.verifyTelegramBindingToken(token, "8576774398", ""); err == nil {
		t.Fatal("verifyTelegramBindingToken() accepted an expired token")
	}
}
