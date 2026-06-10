package handlers

import (
	"context"
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
	statsListLimit       int32 = 5
	statsRecentListLimit int32 = 3

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
		page := parseStatsPage(period)
		text, page, err = formatChatList(database.ChatTypePrivate, page)
		period = strconv.FormatInt(int64(page), 10)
	case statsScreenGroups:
		page := parseStatsPage(period)
		text, page, err = formatChatList(database.ChatTypeGroup, page)
		period = strconv.FormatInt(int64(page), 10)
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

	message := fmt.Sprintf("<b>📊 EaDownloader Analitik</b>\nDönem: %s\n\n", periodText)
	message += fmt.Sprintf("<b>👤 Kullanıcılar:</b> %d\n", stats.TotalPrivateChats)
	message += fmt.Sprintf("<b>👥 Gruplar:</b> %d\n", stats.TotalGroupChats)
	message += fmt.Sprintf("<b>📥 İndirmeler:</b> %d\n", stats.TotalDownloads)
	message += fmt.Sprintf("<b>💾 Toplam boyut:</b> %s\n", formatBytes(stats.TotalDownloadsSize))

	recentUsers, err := formatRecentChatLines(database.ChatTypePrivate, statsRecentListLimit)
	if err != nil {
		return "", err
	}
	if len(recentUsers) > 0 {
		message += "\n<b>👤 Son Özel Kullanıcılar</b>\n" + strings.Join(recentUsers, "\n") + "\n"
	}

	recentGroups, err := formatRecentChatLines(database.ChatTypeGroup, statsRecentListLimit)
	if err != nil {
		return "", err
	}
	if len(recentGroups) > 0 {
		message += "\n<b>👥 Son Gruplar</b>\n" + strings.Join(recentGroups, "\n") + "\n"
	}

	return message, nil
}

func formatChatList(chatType database.ChatType, page int32) (string, int32, error) {
	total, err := database.Q().CountChatsByType(context.Background(), chatType)
	if err != nil {
		return "", page, err
	}
	page = clampStatsPage(page, total)

	chats, err := database.Q().ListChatsByTypePage(
		context.Background(),
		database.ListChatsByTypePageParams{
			Type:        chatType,
			LimitCount:  statsListLimit,
			OffsetCount: statsPageOffset(page),
		},
	)
	if err != nil {
		return "", page, err
	}

	title := "Kullanıcılar"
	if chatType == database.ChatTypeGroup {
		title = "Gruplar"
	}

	if len(chats) == 0 {
		return fmt.Sprintf("<b>%s</b>\n\nHenüz kayıt yok.", title), page, nil
	}

	message := fmt.Sprintf(
		"<b>%s</b>\nToplam: <b>%d</b> · Sayfa: <b>%d/%d</b>\n\n",
		title,
		total,
		page+1,
		totalStatsPages(total),
	)
	for i, chat := range chats {
		message += fmt.Sprintf(
			"<b>%d.</b> %s\n%s\n\nID : <code>%d</code>\n\n",
			int(statsPageOffset(page))+i+1,
			formatAdminPageChatDisplayName(chat),
			formatTimeAgo(chat.LastSeenAt),
			chat.ChatID,
		)
	}
	return strings.TrimSpace(message), page, nil
}

func formatRecentChatLines(chatType database.ChatType, limit int32) ([]string, error) {
	chats, err := database.Q().ListChatsByType(
		context.Background(),
		database.ListChatsByTypeParams{
			Type:       chatType,
			LimitCount: limit,
		},
	)
	if err != nil {
		return nil, err
	}

	lines := make([]string, 0, len(chats))
	for index, chat := range chats {
		lines = append(lines, fmt.Sprintf(
			"%d. %s · %s",
			index+1,
			formatAdminChatDisplayName(chat),
			formatTimeAgo(chat.LastSeenAt),
		))
	}
	return lines, nil
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
		return fmt.Sprintf("<b>🧩 Platformlar</b>\nDönem: %s\n\nHenüz indirme yok.", periodText), nil
	}

	message := fmt.Sprintf("<b>🧩 Platformlar</b>\nDönem: %s\n\n", periodText)
	for i, row := range rows {
		message += fmt.Sprintf(
			"<b>%d. %s</b>\nİndirme: %d\nBoyut: %s\n\n",
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
		return "<b>🚨 Son Hatalar</b>\n\nKayıtlı hata yok.", nil
	}

	message := "<b>🚨 Son Hatalar</b>\n\n"
	for i, row := range rows {
		message += fmt.Sprintf(
			"<b>%d. <code>%s</code></b>\nTekrar: %d\nSon görülme: %s\n%s\n\n",
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
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, 5)
	if screen == statsScreenUsers || screen == statsScreenGroups {
		chatType := database.ChatTypePrivate
		if screen == statsScreenGroups {
			chatType = database.ChatTypeGroup
		}
		if total, err := database.Q().CountChatsByType(context.Background(), chatType); err == nil {
			buttons = append(buttons, statsPaginationRow(screen, parseStatsPage(period), total))
		}
		period = statsPeriodAll
	} else {
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			statsPeriodButton("24h", "1d", screen),
			statsPeriodButton("7d", "7d", screen),
			statsPeriodButton("30d", "30d", screen),
			statsPeriodButton("all", "all", screen),
		})
	}

	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		{Text: "👤 Kullanıcılar", CallbackData: statsCallbackPrefix + statsScreenUsers},
		{Text: "👥 Gruplar", CallbackData: statsCallbackPrefix + statsScreenGroups},
	})
	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		{Text: "🧩 Platformlar", CallbackData: statsCallbackPrefix + statsScreenPlatforms + ":" + period},
		{Text: "🚨 Hatalar", CallbackData: statsCallbackPrefix + statsScreenErrors},
	})
	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		{Text: "📊 Özet", CallbackData: statsCallbackPrefix + statsScreenSummary + ":" + period},
		{Text: "⚙️ Admin", CallbackData: adminCallbackPrefix + adminScreenHome},
	})

	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func statsPaginationRow(screen string, page int32, total int64) []gotgbot.InlineKeyboardButton {
	page = clampStatsPage(page, total)
	totalPages := totalStatsPages(total)
	if totalPages <= 1 {
		return []gotgbot.InlineKeyboardButton{
			{Text: "İlk sayfa", CallbackData: statsCallbackPrefix + screen + ":0"},
			{Text: "1/1", CallbackData: statsCallbackPrefix + screen + ":0"},
			{Text: "Son sayfa", CallbackData: statsCallbackPrefix + screen + ":0"},
		}
	}

	currentPage := strconv.FormatInt(int64(page), 10)
	previousButton := gotgbot.InlineKeyboardButton{
		Text:         "İlk sayfa",
		CallbackData: statsCallbackPrefix + screen + ":" + currentPage,
	}
	if page > 0 {
		previousButton = gotgbot.InlineKeyboardButton{
			Text:         "⬅️ Önceki",
			CallbackData: statsCallbackPrefix + screen + ":" + strconv.FormatInt(int64(page-1), 10),
		}
	}

	nextButton := gotgbot.InlineKeyboardButton{
		Text:         "Son sayfa",
		CallbackData: statsCallbackPrefix + screen + ":" + currentPage,
	}
	if page+1 < totalPages {
		nextButton = gotgbot.InlineKeyboardButton{
			Text:         "Sonraki ➡️",
			CallbackData: statsCallbackPrefix + screen + ":" + strconv.FormatInt(int64(page+1), 10),
		}
	}

	return []gotgbot.InlineKeyboardButton{
		previousButton,
		{Text: fmt.Sprintf("%d/%d", page+1, totalPages), CallbackData: statsCallbackPrefix + screen + ":" + currentPage},
		nextButton,
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

func parseStatsPage(value string) int32 {
	page, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32)
	if err != nil || page < 0 {
		return 0
	}
	return int32(page)
}

func clampStatsPage(page int32, total int64) int32 {
	totalPages := totalStatsPages(total)
	if totalPages == 0 {
		return 0
	}
	if page >= totalPages {
		return totalPages - 1
	}
	return page
}

func totalStatsPages(total int64) int32 {
	if total <= 0 {
		return 1
	}
	return int32((total + int64(statsListLimit) - 1) / int64(statsListLimit))
}

func statsPageOffset(page int32) int32 {
	return page * statsListLimit
}

func statsPeriod(period string) (time.Time, string) {
	now := time.Now()
	switch period {
	case "1d":
		return now.Add(-24 * time.Hour), "24 saat"
	case "7d":
		return now.Add(-7 * 24 * time.Hour), "7 gün"
	case "30d":
		return now.Add(-30 * 24 * time.Hour), "30 gün"
	default:
		return now.Add(-100 * 365 * 24 * time.Hour), "tüm zamanlar"
	}
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
		return "az önce"
	case duration < time.Hour:
		return fmt.Sprintf("%d dk önce", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%d sa önce", int(duration.Hours()))
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
