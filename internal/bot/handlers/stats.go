package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"eadownloader/internal/database"
	"eadownloader/internal/util"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	statsListLimit int32 = 10

	statsScreenSummary   = "summary"
	statsScreenUsers     = "users"
	statsScreenGroups    = "groups"
	statsScreenPlatforms = "platforms"
	statsScreenErrors    = "errors"

	statsPeriodAll = "all"

	statsCallbackPrefix = "stats:"
)

func StatsHandler(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !util.IsBotAdmin(ctx) {
		return ext.EndGroups
	}

	text, err := formatStatsSummary(statsPeriodAll)
	if err != nil {
		return err
	}

	ctx.EffectiveMessage.Reply(
		bot,
		text,
		&gotgbot.SendMessageOpts{
			ParseMode:   gotgbot.ParseModeHTML,
			ReplyMarkup: getStatsKeyboard(statsScreenSummary, statsPeriodAll),
		},
	)
	return ext.EndGroups
}

func StatsCallbackHandler(bot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || !util.IsAdminID(ctx.CallbackQuery.From.Id) {
		return ext.EndGroups
	}

	text, screen, period, err := resolveStatsCallback(ctx.CallbackQuery.Data)
	if err != nil {
		return err
	}

	ctx.CallbackQuery.Answer(bot, nil)
	ctx.EffectiveMessage.EditText(
		bot,
		text,
		&gotgbot.EditMessageTextOpts{
			ParseMode:   gotgbot.ParseModeHTML,
			ReplyMarkup: getStatsKeyboard(screen, period),
		},
	)
	return nil
}

func resolveStatsCallback(data string) (string, string, string, error) {
	parts := strings.Split(data, ":")
	if len(parts) == 2 {
		if isStatsScreen(parts[1]) {
			return resolveStatsScreen(parts[1], statsPeriodAll)
		}
		text, err := formatStatsSummary(parts[1])
		return text, statsScreenSummary, parts[1], err
	}
	if len(parts) < 2 {
		text, err := formatStatsSummary(statsPeriodAll)
		return text, statsScreenSummary, statsPeriodAll, err
	}

	period := statsPeriodAll
	if len(parts) >= 3 {
		period = parts[2]
	}

	return resolveStatsScreen(parts[1], period)
}

func resolveStatsScreen(screen string, period string) (string, string, string, error) {
	var (
		text string
		err  error
	)
	switch screen {
	case statsScreenSummary:
		text, err = formatStatsSummary(period)
	case statsScreenUsers:
		text, err = formatChatList(database.ChatTypePrivate)
		period = statsPeriodAll
	case statsScreenGroups:
		text, err = formatChatList(database.ChatTypeGroup)
		period = statsPeriodAll
	case statsScreenPlatforms:
		text, err = formatPlatformStats(period)
	case statsScreenErrors:
		text, err = formatRecentErrors()
		period = statsPeriodAll
	default:
		text, err = formatStatsSummary(statsPeriodAll)
		screen = statsScreenSummary
		period = statsPeriodAll
	}
	return text, screen, period, err
}

func isStatsScreen(value string) bool {
	switch value {
	case statsScreenSummary, statsScreenUsers, statsScreenGroups, statsScreenPlatforms, statsScreenErrors:
		return true
	default:
		return false
	}
}

func formatStatsSummary(period string) (string, error) {
	sinceDate, periodText := statsPeriod(period)
	stats, err := database.Q().GetStats(
		context.Background(),
		pgtype.Timestamptz{
			Time:  sinceDate,
			Valid: true,
		},
	)
	if err != nil {
		return "", err
	}

	var privateChatsByLang map[string]int64
	var groupChatsByLang map[string]int64
	if err := json.Unmarshal(stats.PrivateChatsByLanguage, &privateChatsByLang); err != nil {
		privateChatsByLang = map[string]int64{}
	}
	if err := json.Unmarshal(stats.GroupChatsByLanguage, &groupChatsByLang); err != nil {
		groupChatsByLang = map[string]int64{}
	}

	message := fmt.Sprintf("<b>EaDownloader stats</b>\nPeriod: %s\n\n", periodText)
	message += fmt.Sprintf("<b>Private users:</b> %d\n", stats.TotalPrivateChats)
	message += fmt.Sprintf("<b>Groups:</b> %d\n", stats.TotalGroupChats)
	message += fmt.Sprintf("<b>Downloads:</b> %d\n", stats.TotalDownloads)
	message += fmt.Sprintf("<b>Total size:</b> %s\n", formatBytes(stats.TotalDownloadsSize))

	if len(privateChatsByLang) > 0 || len(groupChatsByLang) > 0 {
		message += "\n<b>Languages</b>\n"
		if len(privateChatsByLang) > 0 {
			message += "Private: " + formatLanguageMap(privateChatsByLang) + "\n"
		}
		if len(groupChatsByLang) > 0 {
			message += "Groups: " + formatLanguageMap(groupChatsByLang) + "\n"
		}
	}

	return message, nil
}

func formatChatList(chatType database.ChatType) (string, error) {
	chats, err := database.Q().ListChatsByType(
		context.Background(),
		database.ListChatsByTypeParams{
			Type:       chatType,
			LimitCount: statsListLimit,
		},
	)
	if err != nil {
		return "", err
	}

	title := "Private users"
	if chatType == database.ChatTypeGroup {
		title = "Groups"
	}

	if len(chats) == 0 {
		return fmt.Sprintf("<b>%s</b>\n\nNo records yet.", title), nil
	}

	message := fmt.Sprintf("<b>%s</b>\nLast %d active records\n\n", title, len(chats))
	for i, chat := range chats {
		message += fmt.Sprintf(
			"<b>%d. %s</b>\nID: <code>%d</code>\nLanguage: %s\nLast seen: %s\n\n",
			i+1,
			formatChatDisplayName(chat),
			chat.ChatID,
			html.EscapeString(chat.Language),
			formatTimeAgo(chat.LastSeenAt),
		)
	}
	return strings.TrimSpace(message), nil
}

func formatPlatformStats(period string) (string, error) {
	sinceDate, periodText := statsPeriod(period)
	rows, err := database.Q().GetPlatformStats(
		context.Background(),
		pgtype.Timestamptz{
			Time:  sinceDate,
			Valid: true,
		},
	)
	if err != nil {
		return "", err
	}

	if len(rows) == 0 {
		return fmt.Sprintf("<b>Platform stats</b>\nPeriod: %s\n\nNo downloads yet.", periodText), nil
	}

	message := fmt.Sprintf("<b>Platform stats</b>\nPeriod: %s\n\n", periodText)
	for i, row := range rows {
		message += fmt.Sprintf(
			"<b>%d. %s</b>\nDownloads: %d\nSize: %s\n\n",
			i+1,
			html.EscapeString(row.ExtractorID),
			row.Downloads,
			formatBytes(row.TotalSize),
		)
	}
	return strings.TrimSpace(message), nil
}

func formatRecentErrors() (string, error) {
	rows, err := database.Q().GetRecentErrors(context.Background(), statsListLimit)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "<b>Recent errors</b>\n\nNo errors recorded.", nil
	}

	message := "<b>Recent errors</b>\n\n"
	for i, row := range rows {
		message += fmt.Sprintf(
			"<b>%d. <code>%s</code></b>\nOccurrences: %d\nLast seen: %s\n%s\n\n",
			i+1,
			html.EscapeString(row.ID),
			row.Occurrences,
			formatTimestamp(row.LastSeen.Time),
			truncateText(row.Message, 180),
		)
	}
	return strings.TrimSpace(message), nil
}

func getStatsKeyboard(screen string, period string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				statsPeriodButton("24h", "1d", screen),
				statsPeriodButton("7d", "7d", screen),
				statsPeriodButton("30d", "30d", screen),
				statsPeriodButton("all", "all", screen),
			},
			{
				{Text: "Users", CallbackData: statsCallbackPrefix + statsScreenUsers},
				{Text: "Groups", CallbackData: statsCallbackPrefix + statsScreenGroups},
			},
			{
				{Text: "Platforms", CallbackData: statsCallbackPrefix + statsScreenPlatforms + ":" + period},
				{Text: "Errors", CallbackData: statsCallbackPrefix + statsScreenErrors},
			},
			{
				{Text: "Overview", CallbackData: statsCallbackPrefix + statsScreenSummary + ":" + period},
			},
		},
	}
}

func statsPeriodButton(label string, period string, screen string) gotgbot.InlineKeyboardButton {
	targetScreen := screen
	if targetScreen != statsScreenPlatforms {
		targetScreen = statsScreenSummary
	}
	return gotgbot.InlineKeyboardButton{
		Text:         label,
		CallbackData: statsCallbackPrefix + targetScreen + ":" + period,
	}
}

func statsPeriod(period string) (time.Time, string) {
	now := time.Now()
	switch period {
	case "1d":
		return now.Add(-24 * time.Hour), "24 hours"
	case "7d":
		return now.Add(-7 * 24 * time.Hour), "7 days"
	case "30d":
		return now.Add(-30 * 24 * time.Hour), "30 days"
	default:
		return now.Add(-100 * 365 * 24 * time.Hour), "all time"
	}
}

func formatChatDisplayName(chat database.ListChatsByTypeRow) string {
	name := strings.TrimSpace(chat.Title)
	if name == "" {
		name = strings.TrimSpace(strings.Join([]string{chat.FirstName, chat.LastName}, " "))
	}
	if name == "" && chat.Username != "" {
		name = "@" + chat.Username
	}
	if name == "" {
		name = strconv.FormatInt(chat.ChatID, 10)
	}
	if chat.Username != "" && !strings.Contains(name, "@"+chat.Username) {
		name += " (@" + chat.Username + ")"
	}
	return html.EscapeString(name)
}

func formatLanguageMap(values map[string]int64) string {
	parts := make([]string, 0, len(values))
	for lang, count := range values {
		parts = append(parts, fmt.Sprintf("%s: %d", html.EscapeString(lang), count))
	}
	return strings.Join(parts, ", ")
}

func formatBytes(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.2f GB", float64(bytes)/gb)
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/mb)
	case bytes >= kb:
		return fmt.Sprintf("%.0f KB", float64(bytes)/kb)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatTimeAgo(value pgtype.Timestamptz) string {
	if !value.Valid {
		return "unknown"
	}
	return formatTimestamp(value.Time)
}

func formatTimestamp(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	duration := time.Since(value)
	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		return fmt.Sprintf("%d min ago", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(duration.Hours()))
	default:
		return value.Format("2006-01-02 15:04")
	}
}

func truncateText(text string, limit int) string {
	text = html.EscapeString(strings.TrimSpace(text))
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}
