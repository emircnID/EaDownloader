package handlers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"eadownloader/internal/database"
	"eadownloader/internal/util"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/jackc/pgx/v5"
)

func BannedUserHandler(bot *gotgbot.Bot, ctx *ext.Context) error {
	userID, ok := effectiveUserID(ctx)
	if !ok {
		return ext.ContinueGroups
	}
	if user := effectiveUser(ctx); user != nil && !user.IsBot {
		if ctx.EffectiveChat != nil && ctx.EffectiveChat.Type == gotgbot.ChatTypePrivate {
			if _, err := util.PrivateChatFromUser(user); err != nil {
				return err
			}
		}
	}
	if util.IsAdminID(userID) {
		return ext.ContinueGroups
	}

	if groupID, ok := effectiveGroupChatID(ctx); ok {
		if ended, err := stopIfChatRestricted(bot, ctx, groupID); err != nil || ended {
			if err != nil {
				return err
			}
			return ext.EndGroups
		}
	}

	banned, err := database.Q().IsUserBanned(context.Background(), userID)
	if err != nil {
		return err
	}
	if !banned {
		activeMute, err := database.Q().GetActiveMute(context.Background(), userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ext.ContinueGroups
		}
		if err != nil {
			return err
		}
		if ctx.CallbackQuery != nil {
			ctx.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{
				Text:      fmt.Sprintf("Geçici olarak susturuldun. Kalan: %s.", formatDurationLeft(activeMute.ExpiresAt.Time)),
				ShowAlert: true,
			})
		} else if ctx.InlineQuery != nil {
			ctx.InlineQuery.Answer(bot, []gotgbot.InlineQueryResult{}, nil)
		}
		return ext.EndGroups
	}

	if ctx.CallbackQuery != nil {
		ctx.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "Bu botu kullanman engellendi.",
			ShowAlert: true,
		})
	} else if ctx.InlineQuery != nil {
		ctx.InlineQuery.Answer(bot, []gotgbot.InlineQueryResult{}, nil)
	}
	return ext.EndGroups
}

func stopIfChatRestricted(bot *gotgbot.Bot, ctx *ext.Context, chatID int64) (bool, error) {
	banned, err := database.Q().IsUserBanned(context.Background(), chatID)
	if err != nil {
		return false, err
	}
	if banned {
		if ctx.CallbackQuery != nil {
			ctx.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{
				Text:      "Bu grup için bot kullanımı engellendi.",
				ShowAlert: true,
			})
		}
		return true, nil
	}

	activeMute, err := database.Q().GetActiveMute(context.Background(), chatID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if ctx.CallbackQuery != nil {
		ctx.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{
			Text:      fmt.Sprintf("Bu grup geçici olarak susturuldu. Kalan: %s.", formatDurationLeft(activeMute.ExpiresAt.Time)),
			ShowAlert: true,
		})
	}
	return true, nil
}

func formatDurationLeft(expiresAt time.Time) string {
	duration := time.Until(expiresAt)
	if duration <= 0 {
		return "0 dk"
	}
	if duration < time.Hour {
		return fmt.Sprintf("%d dk", int(duration.Minutes())+1)
	}
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%d sa", hours)
	}
	return fmt.Sprintf("%d sa %d dk", hours, minutes)
}

func effectiveUserID(ctx *ext.Context) (int64, bool) {
	switch {
	case ctx.EffectiveUser != nil:
		return ctx.EffectiveUser.Id, true
	case ctx.CallbackQuery != nil:
		return ctx.CallbackQuery.From.Id, true
	case ctx.InlineQuery != nil:
		return ctx.InlineQuery.From.Id, true
	default:
		return 0, false
	}
}

func effectiveUser(ctx *ext.Context) *gotgbot.User {
	switch {
	case ctx.EffectiveUser != nil:
		return ctx.EffectiveUser
	case ctx.CallbackQuery != nil:
		return &ctx.CallbackQuery.From
	case ctx.InlineQuery != nil:
		return &ctx.InlineQuery.From
	default:
		return nil
	}
}

func effectiveGroupChatID(ctx *ext.Context) (int64, bool) {
	if ctx.EffectiveChat == nil || ctx.EffectiveChat.Id >= 0 {
		return 0, false
	}
	return ctx.EffectiveChat.Id, true
}
