package handlers

import (
	"context"
	"fmt"
	"html"
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

	statsScreenSummary         = "summary"
	statsScreenPlatforms       = "platforms"
	statsScreenErrors          = "errors"
	statsScreenRecentDownloads = "recent_downloads"

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
	case statsScreenPlatforms:
		text, err = formatPlatformStats(period)
	case statsScreenRecentDownloads:
		text, err = formatGlobalRecentDownloads()
		period = statsPeriodAll
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
	case statsScreenSummary, statsScreenPlatforms, statsScreenErrors, statsScreenRecentDownloads:
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
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, 4)
	if screen == statsScreenErrors {
		return gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{Text: "🖥 Sistem Paneli", CallbackData: adminCallbackPrefix + adminScreenSystem},
				},
				statsHomeRow(),
			},
		}
	}

	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		statsPeriodButton("24h", "1d", screen),
		statsPeriodButton("7d", "7d", screen),
		statsPeriodButton("30d", "30d", screen),
		statsPeriodButton("all", "all", screen),
	})

	switch screen {
	case statsScreenSummary:
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{Text: "🧩 Platformlar", CallbackData: statsCallbackPrefix + statsScreenPlatforms + ":" + period},
			{Text: "📥 Son İndirmeler", CallbackData: statsCallbackPrefix + statsScreenRecentDownloads + ":" + period},
		})
	case statsScreenPlatforms:
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{Text: "📊 Özet", CallbackData: statsCallbackPrefix + statsScreenSummary + ":" + period},
			{Text: "📥 Son İndirmeler", CallbackData: statsCallbackPrefix + statsScreenRecentDownloads + ":" + period},
		})
	case statsScreenRecentDownloads:
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{Text: "📊 Özet", CallbackData: statsCallbackPrefix + statsScreenSummary + ":" + period},
			{Text: "🧩 Platformlar", CallbackData: statsCallbackPrefix + statsScreenPlatforms + ":" + period},
		})
	}

	buttons = append(buttons, statsHomeRow())

	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func statsHomeRow() []gotgbot.InlineKeyboardButton {
	return []gotgbot.InlineKeyboardButton{
		{Text: "🏠 Ana menü", CallbackData: adminCallbackPrefix + adminScreenHome},
	}
}
func statsPeriodButton(label string, period string, screen string) gotgbot.InlineKeyboardButton {
	targetScreen := screen
	if targetScreen != statsScreenPlatforms && targetScreen != statsScreenRecentDownloads {
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

func formatGlobalRecentDownloads() (string, error) {
	rows, err := database.Conn().Query(context.Background(), `
		SELECT 
			d.extractor_id,
			d.content_url,
			d.item_count,
			d.total_file_size,
			d.created_at,
			d.chat_type,
			COALESCE(c.title, ''),
			COALESCE(c.username, ''),
			COALESCE(c.first_name, ''),
			COALESCE(c.last_name, ''),
			COALESCE(u.username, ''),
			COALESCE(u.first_name, ''),
			COALESCE(u.last_name, '')
		FROM download_events d
		LEFT JOIN chat c ON d.chat_id = c.chat_id
		LEFT JOIN chat u ON d.user_id = u.chat_id
		ORDER BY d.created_at DESC
		LIMIT 5
	`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var lines []string
	count := 0
	for rows.Next() {
		count++
		var (
			extractorID   string
			contentURL    string
			itemCount     int
			totalFileSize int64
			createdAt     time.Time
			chatType      string
			cTitle        string
			cUsername     string
			cFirstName    string
			cLastName     string
			uUsername     string
			uFirstName    string
			uLastName     string
		)
		err = rows.Scan(
			&extractorID, &contentURL, &itemCount, &totalFileSize, &createdAt, &chatType,
			&cTitle, &cUsername, &cFirstName, &cLastName,
			&uUsername, &uFirstName, &uLastName,
		)
		if err != nil {
			return "", err
		}

		// Format user display name
		var userDisp string
		uName := strings.TrimSpace(strings.Join([]string{uFirstName, uLastName}, " "))
		if uName == "" && uUsername != "" {
			uName = "@" + uUsername
		}
		if uName == "" {
			userDisp = "Bilinmeyen Kullanıcı"
		} else {
			userDisp = html.EscapeString(uName)
			if uUsername != "" && !strings.Contains(strings.ToLower(uName), strings.ToLower("@"+uUsername)) {
				userDisp += " @" + html.EscapeString(uUsername)
			}
		}

		// Format source chat display name if group
		var groupDisp string
		if chatType == "group" {
			gName := strings.TrimSpace(cTitle)
			if gName == "" && cUsername != "" {
				gName = "@" + cUsername
			}
			if gName == "" {
				groupDisp = "Bilinmeyen Grup"
			} else {
				groupDisp = html.EscapeString(gName)
				if cUsername != "" && !strings.Contains(strings.ToLower(gName), strings.ToLower("@"+cUsername)) {
					groupDisp += " @" + html.EscapeString(cUsername)
				}
			}
		}

		// Format link - truncate if too long
		displayURL := contentURL
		if len(displayURL) > 30 {
			displayURL = displayURL[:27] + "..."
		}

		platformName := extractorID
		if len(platformName) > 0 {
			platformName = strings.ToUpper(platformName[:1]) + platformName[1:]
		}

		line := fmt.Sprintf("<b>%d. 🧩 %s</b>\n", count, html.EscapeString(platformName))
		line += fmt.Sprintf("   👤 %s\n", userDisp)
		if chatType == "group" {
			line += fmt.Sprintf("   👥 Grup: %s\n", groupDisp)
		}
		line += fmt.Sprintf("   🔗 <a href=\"%s\">%s</a>\n", html.EscapeString(contentURL), html.EscapeString(displayURL))
		line += fmt.Sprintf("   💾 %s · %d adet\n", formatBytes(totalFileSize), itemCount)
		line += fmt.Sprintf("   ⏱ %s", formatTimestamp(createdAt))
		lines = append(lines, line)
	}

	if count == 0 {
		return "<b>📥 Son İndirilenler</b>\n\nHenüz indirme kaydı yok.", nil
	}

	return "<b>📥 Son İndirilenler</b>\n\n" + strings.Join(lines, "\n\n"), nil
}
