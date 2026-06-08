package handlers

import (
	"testing"

	"eadownloader/internal/config"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestTextMentionModerationTarget(t *testing.T) {
	user := &gotgbot.User{
		Id:        6473764544,
		FirstName: "Eclaire",
		LastName:  "Nuage",
		Username:  "EclaireNuage",
	}
	ctx := &ext.Context{
		EffectiveMessage: &gotgbot.Message{
			Text: "/ban Eclaire Nuage",
			Entities: []gotgbot.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 4},
				{Type: "text_mention", Offset: 5, Length: 13, User: user},
			},
		},
	}

	got, ok := textMentionModerationTarget(ctx)
	if !ok {
		t.Fatal("expected text mention target")
	}
	if got.Id != user.Id {
		t.Fatalf("expected user ID %d, got %d", user.Id, got.Id)
	}
}

func TestParseUserIDTarget(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int64
	}{
		{name: "plain ID", value: "8649884827", want: 8649884827},
		{name: "tg user link", value: "tg://user?id=8649884827", want: 8649884827},
		{name: "telegram user link", value: "telegram://user?id=8649884827", want: 8649884827},
		{name: "open message link", value: "tg://openmessage?user_id=8649884827", want: 8649884827},
		{name: "angle wrapped link", value: "<tg://user?id=8649884827>", want: 8649884827},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := parseUserIDTarget(tt.value)
			if err != nil {
				t.Fatalf("parseUserIDTarget returned error: %v", err)
			}
			if !ok {
				t.Fatal("expected target to parse")
			}
			if got != tt.want {
				t.Fatalf("expected user ID %d, got %d", tt.want, got)
			}
		})
	}
}

func TestIsModerationAdminAllowsAdminsInGroups(t *testing.T) {
	previousAdmins := config.Env.Admins
	config.Env.Admins = []int64{12345}
	t.Cleanup(func() {
		config.Env.Admins = previousAdmins
	})

	ctx := &ext.Context{
		EffectiveChat: &gotgbot.Chat{Type: gotgbot.ChatTypeSupergroup},
		EffectiveUser: &gotgbot.User{Id: 12345},
		EffectiveMessage: &gotgbot.Message{
			From: &gotgbot.User{Id: 12345},
		},
	}

	if !isModerationAdmin(ctx) {
		t.Fatal("expected configured admin to be allowed in group")
	}
}

func TestIsModerationAdminRejectsNonAdmins(t *testing.T) {
	previousAdmins := config.Env.Admins
	config.Env.Admins = []int64{12345}
	t.Cleanup(func() {
		config.Env.Admins = previousAdmins
	})

	ctx := &ext.Context{
		EffectiveChat: &gotgbot.Chat{Type: gotgbot.ChatTypeSupergroup},
		EffectiveUser: &gotgbot.User{Id: 67890},
		EffectiveMessage: &gotgbot.Message{
			From: &gotgbot.User{Id: 67890},
		},
	}

	if isModerationAdmin(ctx) {
		t.Fatal("expected non-admin to be rejected")
	}
}
