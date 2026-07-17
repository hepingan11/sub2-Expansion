package app

import (
	"encoding/json"
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

func TestTelegramAlreadyBoundMessage(t *testing.T) {
	got := telegramAlreadyBoundMessage(123, "https://sub2.example.com/")
	want := "这个 Telegram 账号已经绑定 Sub2API 用户 123。\n前端公开地址：\nhttps://sub2.example.com"
	if got != want {
		t.Fatalf("telegramAlreadyBoundMessage() = %q, want %q", got, want)
	}
}

func TestTelegramAlreadyBoundMessageWithoutFrontendURL(t *testing.T) {
	got := telegramAlreadyBoundMessage(123, "")
	want := "这个 Telegram 账号已经绑定 Sub2API 用户 123。\n前端公开地址尚未配置。"
	if got != want {
		t.Fatalf("telegramAlreadyBoundMessage() = %q, want %q", got, want)
	}
}

func TestTelegramBindingCompletedMessageWithInvitation(t *testing.T) {
	invitation := &InvitationBindingResult{
		Bound:         true,
		InviteCode:    "ABCDEFGH",
		RewardAmount:  MustAmount("5.00"),
		InviterUserID: 456,
	}
	got := telegramBindingCompletedMessage(123, "https://sub2.example.com/", true, invitation)
	want := "Telegram 账号绑定成功。\nSub2API 用户：123\n\n邀请关系建立成功。\n邀请码：ABCDEFGH\n邀请奖励已发放给邀请人。\n\n前端公开地址：\nhttps://sub2.example.com"
	if got != want {
		t.Fatalf("telegramBindingCompletedMessage() = %q, want %q", got, want)
	}

	inviterMessage := telegramInvitationSucceededMessage(*invitation)
	wantInviterMessage := "邀请成功。\n邀请码：ABCDEFGH\n邀请奖励：5.00\n奖励已发放到你的 Sub2API 余额。"
	if inviterMessage != wantInviterMessage {
		t.Fatalf("telegramInvitationSucceededMessage() = %q, want %q", inviterMessage, wantInviterMessage)
	}
}

func TestInvitationBindingResultHidesInternalInviterUserID(t *testing.T) {
	raw, err := json.Marshal(InvitationBindingResult{Bound: true, InviterUserID: 456})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(raw), "456") || strings.Contains(string(raw), "inviter") {
		t.Fatalf("InvitationBindingResult JSON exposed internal inviter user ID: %s", raw)
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
