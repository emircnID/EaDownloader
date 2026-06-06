package handlers

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"eadownloader/internal/config"
	"eadownloader/internal/database"
	"eadownloader/internal/util"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	adminCallbackPrefix = "admin:"

	adminScreenHome   = "home"
	adminScreenSystem = "system"
	adminScreenBans   = "bans"

	adminActionBan        = "ban"
	adminActionBanConfirm = "ban_confirm"
	adminActionUnban      = "unban"
)

func AdminHandler(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !util.IsBotAdmin(ctx) {
		return ext.EndGroups
	}

	ctx.EffectiveMessage.Reply(
		bot,
		formatAdminHome(),
		&gotgbot.SendMessageOpts{
			ParseMode:   gotgbot.ParseModeHTML,
			ReplyMarkup: getAdminKeyboard(),
		},
	)
	return ext.EndGroups
}

func AdminCallbackHandler(bot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || !util.IsAdminID(ctx.CallbackQuery.From.Id) {
		return ext.EndGroups
	}

	text, keyboard, err := resolveAdminCallback(ctx)
	if err != nil {
		return err
	}

	ctx.CallbackQuery.Answer(bot, nil)
	ctx.EffectiveMessage.EditText(
		bot,
		text,
		&gotgbot.EditMessageTextOpts{
			ParseMode:   gotgbot.ParseModeHTML,
			ReplyMarkup: keyboard,
		},
	)
	return nil
}

func resolveAdminCallback(ctx *ext.Context) (string, gotgbot.InlineKeyboardMarkup, error) {
	data := strings.TrimPrefix(ctx.CallbackQuery.Data, adminCallbackPrefix)
	text := formatAdminHome()
	keyboard := getAdminKeyboard()

	switch {
	case data == adminScreenSystem:
		var err error
		text, err = formatAdminSystem()
		keyboard = getAdminSystemKeyboard()
		return text, keyboard, err
	case data == adminScreenBans:
		return formatAdminBans()
	case strings.HasPrefix(data, adminActionBanConfirm+":"):
		return banUserFromCallback(ctx, strings.TrimPrefix(data, adminActionBanConfirm+":"))
	case strings.HasPrefix(data, adminActionBan+":"):
		return formatAdminBanConfirm(strings.TrimPrefix(data, adminActionBan+":"))
	case strings.HasPrefix(data, adminActionUnban+":"):
		return unbanUserFromCallback(strings.TrimPrefix(data, adminActionUnban+":"))
	default:
		return text, keyboard, nil
	}
}

func formatAdminHome() string {
	return "<b>EaDownloader admin</b>\n\n" +
		"Control center. Stats are kept in their own detailed screen; user bans live here."
}

func formatAdminSystem() (string, error) {
	bannedUsersCount, err := database.Q().CountBannedUsers(context.Background())
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"<b>System</b>\n\n"+
			"Admins: %d\n"+
			"Whitelist: %d\n"+
			"Banned users: %d\n"+
			"Concurrent updates: %d\n"+
			"Max duration: %s\n"+
			"Max file size: %s\n"+
			"Caching: %t\n"+
			"Log level: %s\n"+
			"Time: %s",
		len(config.Env.Admins),
		len(config.Env.Whitelist),
		bannedUsersCount,
		config.Env.ConcurrentUpdates,
		config.Env.MaxDuration,
		formatBytes(config.Env.MaxFileSize),
		config.Env.Caching,
		config.Env.LogLevel.String(),
		time.Now().Format("2006-01-02 15:04:05"),
	), nil
}

func formatAdminBans() (string, gotgbot.InlineKeyboardMarkup, error) {
	bannedUsersCount, err := database.Q().CountBannedUsers(context.Background())
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	rows, err := database.Q().ListBannedUsers(context.Background(), statsListLimit)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	message := fmt.Sprintf("<b>Banned users</b>\nTotal: %d\n\n", bannedUsersCount)
	if len(rows) == 0 {
		message += "No banned users yet."
	} else {
		for i, row := range rows {
			message += fmt.Sprintf(
				"<b>%d. %s</b>\nID: <code>%d</code>\nReason: %s\nBanned by: <code>%d</code>\nSince: %s\n\n",
				i+1,
				formatBannedUserDisplayName(row),
				row.UserID,
				html.EscapeString(row.Reason),
				row.BannedBy,
				formatTimeAgo(row.CreatedAt),
			)
		}
	}

	return strings.TrimSpace(message), getAdminBansKeyboard(rows), nil
}

func formatAdminBanConfirm(value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return formatAdminBans()
	}

	message := fmt.Sprintf(
		"<b>Ban user?</b>\n\nUser ID: <code>%d</code>\n\nThis user will not be able to use the bot in private chats, groups or inline mode.",
		userID,
	)
	return message, gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "Confirm ban", CallbackData: adminCallbackPrefix + adminActionBanConfirm + ":" + strconv.FormatInt(userID, 10)},
			},
			{
				{Text: "Back", CallbackData: statsCallbackPrefix + statsScreenUsers},
				{Text: "Close", CallbackData: "close"},
			},
		},
	}, nil
}

func banUserFromCallback(ctx *ext.Context, value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return formatAdminBans()
	}
	if util.IsAdminID(userID) {
		return "<b>Ban skipped</b>\n\nAdmins cannot be banned.", getAdminSystemKeyboard(), nil
	}

	_, err = database.Q().BanUser(
		context.Background(),
		database.BanUserParams{
			UserID:   userID,
			Reason:   "admin panel",
			BannedBy: ctx.CallbackQuery.From.Id,
		},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	return formatAdminBans()
}

func unbanUserFromCallback(value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return formatAdminBans()
	}
	if err := database.Q().UnbanUser(context.Background(), userID); err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	return formatAdminBans()
}

func getAdminKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "Stats", CallbackData: statsCallbackPrefix + statsScreenSummary + ":" + statsPeriodAll},
				{Text: "System", CallbackData: adminCallbackPrefix + adminScreenSystem},
			},
			{
				{Text: "Ban Users", CallbackData: statsCallbackPrefix + statsScreenUsers},
				{Text: "Banned Users", CallbackData: adminCallbackPrefix + adminScreenBans},
			},
			{
				{Text: "Close", CallbackData: "close"},
			},
		},
	}
}

func getAdminSystemKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "Back", CallbackData: adminCallbackPrefix + adminScreenHome},
				{Text: "Close", CallbackData: "close"},
			},
		},
	}
}

func getAdminBansKeyboard(rows []database.ListBannedUsersRow) gotgbot.InlineKeyboardMarkup {
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, len(rows)+1)
	for _, row := range rows {
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{
				Text:         "Unban " + formatBannedUserButtonLabel(row),
				CallbackData: adminCallbackPrefix + adminActionUnban + ":" + strconv.FormatInt(row.UserID, 10),
			},
		})
	}
	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		{Text: "Back", CallbackData: adminCallbackPrefix + adminScreenHome},
		{Text: "Close", CallbackData: "close"},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func getRecentUserBanKeyboardRows() [][]gotgbot.InlineKeyboardButton {
	rows, err := database.Q().ListChatsByType(
		context.Background(),
		database.ListChatsByTypeParams{
			Type:       database.ChatTypePrivate,
			LimitCount: 5,
		},
	)
	if err != nil {
		return nil
	}

	buttons := make([][]gotgbot.InlineKeyboardButton, 0, len(rows))
	for _, row := range rows {
		if util.IsAdminID(row.ChatID) {
			continue
		}
		action := adminActionBan
		prefix := "Ban "
		if banned, err := database.Q().IsUserBanned(context.Background(), row.ChatID); err == nil && banned {
			action = adminActionUnban
			prefix = "Unban "
		}
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{
				Text:         prefix + truncateButtonText(chatDisplayLabel(row), 40),
				CallbackData: adminCallbackPrefix + action + ":" + strconv.FormatInt(row.ChatID, 10),
			},
		})
	}
	return buttons
}

func formatBannedUserDisplayName(row database.ListBannedUsersRow) string {
	name := bannedUserDisplayLabel(row)
	return fmt.Sprintf(
		"<a href='tg://user?id=%d'>%s</a>",
		row.UserID,
		html.EscapeString(name),
	)
}

func formatBannedUserButtonLabel(row database.ListBannedUsersRow) string {
	return truncateButtonText(bannedUserDisplayLabel(row), 32)
}

func bannedUserDisplayLabel(row database.ListBannedUsersRow) string {
	name := strings.TrimSpace(joinValidTexts(row.FirstName, row.LastName))
	if name == "" && row.Username.Valid && row.Username.String != "" {
		name = "@" + row.Username.String
	}
	if name == "" {
		name = strconv.FormatInt(row.UserID, 10)
	}
	return name
}

func joinValidTexts(values ...pgtype.Text) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value.Valid && strings.TrimSpace(value.String) != "" {
			parts = append(parts, strings.TrimSpace(value.String))
		}
	}
	return strings.Join(parts, " ")
}

func truncateButtonText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	return text[:limit-3] + "..."
}
