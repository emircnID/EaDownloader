package handlers

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"eadownloader/internal/database"
	"eadownloader/internal/util"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func BanCommandHandler(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !util.IsBotAdmin(ctx) {
		return ext.EndGroups
	}

	userID, label, err := resolveModerationTarget(ctx, commandArgs(ctx), 0)
	if err != nil {
		return replyModerationUsage(bot, ctx, "Kullanım: <code>/ban 123456</code>, <code>/ban @username</code> veya kullanıcı mesajına reply.")
	}
	if util.IsAdminID(userID) {
		return replyModerationUsage(bot, ctx, "Adminler banlanamaz.")
	}

	_, err = database.Q().BanUser(
		context.Background(),
		database.BanUserParams{
			UserID:   userID,
			Reason:   "admin command",
			BannedBy: ctx.EffectiveUser.Id,
		},
	)
	if err != nil {
		return err
	}

	return replyModerationUsage(bot, ctx, fmt.Sprintf("⛔ <b>%s</b> banlandı.\nID: <code>%d</code>", html.EscapeString(label), userID))
}

func UnbanCommandHandler(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !util.IsBotAdmin(ctx) {
		return ext.EndGroups
	}

	userID, label, err := resolveModerationTarget(ctx, commandArgs(ctx), 0)
	if err != nil {
		return replyModerationUsage(bot, ctx, "Kullanım: <code>/unban 123456</code>, <code>/unban @username</code> veya kullanıcı mesajına reply.")
	}

	if err := database.Q().UnbanUser(context.Background(), userID); err != nil {
		return err
	}
	if err := database.Q().UnmuteUser(context.Background(), userID); err != nil {
		return err
	}

	return replyModerationUsage(bot, ctx, fmt.Sprintf("✅ <b>%s</b> için ban/susturma kaldırıldı.\nID: <code>%d</code>", html.EscapeString(label), userID))
}

func SilentCommandHandler(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !util.IsBotAdmin(ctx) {
		return ext.EndGroups
	}

	args := commandArgs(ctx)
	duration, err := parseSilentArgs(args)
	if err != nil {
		return replyModerationUsage(bot, ctx, "Kullanım: <code>/silent 1h 123456</code>, <code>/silent 30m @username</code> veya reply ile <code>/silent 1h</code>.")
	}

	userID, label, err := resolveModerationTarget(ctx, args, 0)
	if err != nil {
		return replyModerationUsage(bot, ctx, "Susturulacak kullanıcı bulunamadı. ID, @username ya da reply kullan.")
	}
	if util.IsAdminID(userID) {
		return replyModerationUsage(bot, ctx, "Adminler susturulamaz.")
	}

	expiresAt := time.Now().Add(duration)
	err = database.Q().MuteUser(
		context.Background(),
		database.MuteUserParams{
			UserID:    userID,
			Reason:    "admin command",
			MutedBy:   ctx.EffectiveUser.Id,
			ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		},
	)
	if err != nil {
		return err
	}

	return replyModerationUsage(
		bot,
		ctx,
		fmt.Sprintf("🔇 <b>%s</b> susturuldu.\nID: <code>%d</code>\nSüre: <b>%s</b>", html.EscapeString(label), userID, formatCommandDuration(duration)),
	)
}

func commandArgs(ctx *ext.Context) []string {
	if ctx.EffectiveMessage == nil {
		return nil
	}
	fields := strings.Fields(ctx.EffectiveMessage.Text)
	if len(fields) <= 1 {
		return nil
	}
	return fields[1:]
}

func parseSilentArgs(args []string) (time.Duration, error) {
	for _, arg := range args {
		duration, err := parseCommandDuration(arg)
		if err == nil {
			return duration, nil
		}
	}
	return 0, errors.New("duration not found")
}

func parseCommandDuration(value string) (time.Duration, error) {
	duration, err := time.ParseDuration(strings.ToLower(strings.TrimSpace(value)))
	if err != nil {
		return 0, err
	}
	if duration < time.Minute || duration > 30*24*time.Hour {
		return 0, errors.New("duration out of range")
	}
	return duration, nil
}

func resolveModerationTarget(ctx *ext.Context, args []string, startIndex int) (int64, string, error) {
	for index := startIndex; index < len(args); index++ {
		candidate := strings.TrimSpace(args[index])
		if candidate == "" {
			continue
		}
		if _, err := parseCommandDuration(candidate); err == nil {
			continue
		}
		userID, label, err := resolveModerationTargetValue(candidate)
		if err == nil {
			return userID, label, nil
		}
	}

	if ctx.EffectiveMessage != nil &&
		ctx.EffectiveMessage.ReplyToMessage != nil &&
		ctx.EffectiveMessage.ReplyToMessage.From != nil {
		user := ctx.EffectiveMessage.ReplyToMessage.From
		return user.Id, userDisplayLabel(user), nil
	}

	return 0, "", errors.New("target not found")
}

func resolveModerationTargetValue(value string) (int64, string, error) {
	if strings.HasPrefix(value, "@") {
		username := strings.TrimPrefix(value, "@")
		chat, err := database.Q().GetPrivateChatByUsername(context.Background(), username)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", fmt.Errorf("username not found: %s", username)
		}
		if err != nil {
			return 0, "", err
		}
		return chat.ChatID, privateChatDisplayLabel(chat.FirstName, chat.LastName, chat.Username, chat.ChatID), nil
	}

	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, "", err
	}
	return userID, strconv.FormatInt(userID, 10), nil
}

func privateChatDisplayLabel(firstName string, lastName string, username string, chatID int64) string {
	name := strings.TrimSpace(strings.Join([]string{firstName, lastName}, " "))
	if name == "" && username != "" {
		name = "@" + username
	}
	if name == "" {
		name = strconv.FormatInt(chatID, 10)
	}
	return name
}

func userDisplayLabel(user *gotgbot.User) string {
	name := strings.TrimSpace(strings.Join([]string{user.FirstName, user.LastName}, " "))
	if name == "" && user.Username != "" {
		name = "@" + user.Username
	}
	if name == "" {
		name = strconv.FormatInt(user.Id, 10)
	}
	return name
}

func formatCommandDuration(duration time.Duration) string {
	if duration < time.Hour {
		return fmt.Sprintf("%d dk", int(duration.Minutes()))
	}
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%d saat", hours)
	}
	return fmt.Sprintf("%d saat %d dk", hours, minutes)
}

func replyModerationUsage(bot *gotgbot.Bot, ctx *ext.Context, text string) error {
	_, err := ctx.EffectiveMessage.Reply(
		bot,
		text,
		&gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML},
	)
	return err
}
