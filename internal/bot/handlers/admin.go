package handlers

import (
	"context"
	"errors"
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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	adminCallbackPrefix = "admin:"

	adminScreenHome       = "home"
	adminScreenModeration = "moderation"
	adminScreenSystem     = "system"
	adminScreenUsers      = "users"
	adminScreenGroups     = "groups"
	adminScreenBans       = "bans"
	adminScreenMutes      = "mutes"
	adminScreenUser       = "user"
	adminScreenGroup      = "group"

	adminActionBanConfirm = "ban_confirm"
	adminActionBan        = "ban"
	adminActionUnban      = "unban"
	adminActionMute       = "mute"
	adminActionUnmute     = "unmute"

	adminPageSize      int32 = 5
	adminActivityLimit int32 = 5
)

func AdminHandler(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !util.IsBotAdmin(ctx) {
		return ext.EndGroups
	}

	text, keyboard, err := buildAdminHome()
	if err != nil {
		return err
	}

	ctx.EffectiveMessage.Reply(
		bot,
		text,
		&gotgbot.SendMessageOpts{
			ParseMode:   gotgbot.ParseModeHTML,
			ReplyMarkup: keyboard,
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

	switch {
	case data == adminScreenHome:
		return buildAdminHome()
	case data == adminScreenModeration:
		return buildModerationHome()
	case data == adminScreenUsers:
		return buildUserList()
	case strings.HasPrefix(data, adminScreenUsers+":"):
		return buildUserList(strings.TrimPrefix(data, adminScreenUsers+":"))
	case data == adminScreenGroups:
		return buildGroupList()
	case strings.HasPrefix(data, adminScreenGroups+":"):
		return buildGroupList(strings.TrimPrefix(data, adminScreenGroups+":"))
	case data == adminScreenBans:
		return buildBannedUserList()
	case data == adminScreenMutes:
		return buildMutedUserList()
	case data == adminScreenSystem:
		return buildSystemPanel()
	case strings.HasPrefix(data, adminScreenUser+":"):
		return buildUserProfile(strings.TrimPrefix(data, adminScreenUser+":"))
	case strings.HasPrefix(data, adminScreenGroup+":"):
		return buildGroupProfile(strings.TrimPrefix(data, adminScreenGroup+":"))
	case strings.HasPrefix(data, adminActionBanConfirm+":"):
		return buildBanConfirm(strings.TrimPrefix(data, adminActionBanConfirm+":"))
	case strings.HasPrefix(data, adminActionBan+":"):
		return banUserFromCallback(ctx, strings.TrimPrefix(data, adminActionBan+":"))
	case strings.HasPrefix(data, adminActionUnban+":"):
		return unbanUserFromCallback(strings.TrimPrefix(data, adminActionUnban+":"))
	case strings.HasPrefix(data, adminActionMute+":"):
		return muteUserFromCallback(ctx, strings.TrimPrefix(data, adminActionMute+":"))
	case strings.HasPrefix(data, adminActionUnmute+":"):
		return unmuteUserFromCallback(strings.TrimPrefix(data, adminActionUnmute+":"))
	default:
		return buildAdminHome()
	}
}

func buildAdminHome() (string, gotgbot.InlineKeyboardMarkup, error) {
	stats, err := database.Q().GetStats(
		context.Background(),
		pgtype.Timestamptz{
			Time:  time.Now().Add(-100 * 365 * 24 * time.Hour),
			Valid: true,
		},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	bannedUsersCount, err := database.Q().CountBannedUsers(context.Background())
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	mutedUsersCount, err := database.Q().CountActiveMutedUsers(context.Background())
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	text := fmt.Sprintf(
		"<b>⚙️ EaDownloader Admin</b>\n"+
			"<i>Operasyon paneli</i>\n\n"+
			"<b>📊 Genel Durum</b>\n"+
			"%s\n"+
			"%s\n"+
			"%s\n"+
			"%s\n\n"+
			"💾 Toplam boyut: <b>%s</b>\n"+
			"🔇 Susturulan: <b>%d</b>\n\n"+
			"Bir bölüm seçin.",
		metricBar("👤 Kullanıcılar", stats.TotalPrivateChats, max(stats.TotalPrivateChats, stats.TotalGroupChats)),
		metricBar("👥 Gruplar", stats.TotalGroupChats, max(stats.TotalPrivateChats, stats.TotalGroupChats)),
		metricBar("📥 İndirmeler", stats.TotalDownloads, stats.TotalDownloads),
		metricBar("⛔ Banlı", bannedUsersCount, max(stats.TotalPrivateChats, 1)),
		formatBytes(stats.TotalDownloadsSize),
		mutedUsersCount,
	)

	return text, gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "👤 Kullanıcılar", CallbackData: adminCallbackPrefix + adminScreenUsers},
				{Text: "👥 Gruplar", CallbackData: adminCallbackPrefix + adminScreenGroups},
			},
			{
				{Text: "📊 Analitik", CallbackData: statsCallbackPrefix + statsScreenSummary + ":" + statsPeriodAll},
				{Text: "🚨 Hatalar", CallbackData: statsCallbackPrefix + statsScreenErrors},
			},
			{
				{Text: "🛡 Moderasyon", CallbackData: adminCallbackPrefix + adminScreenModeration},
				{Text: "🖥 Sistem", CallbackData: adminCallbackPrefix + adminScreenSystem},
			},
		},
	}, nil
}

func metricBar(label string, value int64, maxValue int64) string {
	const width = 8
	if maxValue <= 0 {
		maxValue = 1
	}
	filled := int((value*width + maxValue - 1) / maxValue)
	if value == 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return fmt.Sprintf(
		"%s  <code>%s%s</code> <b>%d</b>",
		label,
		strings.Repeat("█", filled),
		strings.Repeat("░", width-filled),
		value,
	)
}

func buildModerationHome() (string, gotgbot.InlineKeyboardMarkup, error) {
	bannedUsersCount, err := database.Q().CountBannedUsers(context.Background())
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	mutedUsersCount, err := database.Q().CountActiveMutedUsers(context.Background())
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	text := fmt.Sprintf(
		"<b>🛡 Moderasyon</b>\n\n"+
			"Kullanıcıları profil kartı üzerinden yönetin.\n"+
			"Banlı kullanıcı: <b>%d</b>\n"+
			"Susturulan kullanıcı: <b>%d</b>",
		bannedUsersCount,
		mutedUsersCount,
	)

	return text, gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "👤 Kullanıcılar", CallbackData: adminCallbackPrefix + adminScreenUsers},
				{Text: "⛔ Banlılar", CallbackData: adminCallbackPrefix + adminScreenBans},
			},
			{
				{Text: "🔇 Susturulanlar", CallbackData: adminCallbackPrefix + adminScreenMutes},
			},
			{
				{Text: "⬅️ Geri", CallbackData: adminCallbackPrefix + adminScreenHome},
			},
		},
	}, nil
}

func buildUserList(pageValues ...string) (string, gotgbot.InlineKeyboardMarkup, error) {
	page := parseAdminPage(pageValues...)
	total, err := database.Q().CountChatsByType(context.Background(), database.ChatTypePrivate)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	page = clampAdminPage(page, total)

	rows, err := database.Q().ListChatsByTypePage(
		context.Background(),
		database.ListChatsByTypePageParams{
			Type:        database.ChatTypePrivate,
			LimitCount:  adminPageSize,
			OffsetCount: pageOffset(page),
		},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	if len(rows) == 0 {
		return "<b>👤 Kullanıcılar</b>\n\nHenüz kayıtlı kullanıcı yok.", userListKeyboard(rows, page, total), nil
	}

	text := fmt.Sprintf(
		"<b>👤 Kullanıcılar</b>\n"+
			"Toplam: <b>%d</b> · Sayfa: <b>%d/%d</b>\n\n",
		total,
		page+1,
		totalAdminPages(total),
	)
	for index, row := range rows {
		status := "Aktif"
		if banned, err := database.Q().IsUserBanned(context.Background(), row.ChatID); err == nil && banned {
			status = "Banlı"
		} else if activeMute, err := database.Q().GetActiveMute(context.Background(), row.ChatID); err == nil {
			status = "Susturuldu: " + formatDurationLeft(activeMute.ExpiresAt.Time)
		}
		text += fmt.Sprintf(
			"<b>%d.</b> %s\n%s · %s\n\nID : <code>%d</code>\n\n",
			int(pageOffset(page))+index+1,
			formatAdminPageChatDisplayName(row),
			status,
			formatTimeAgo(row.LastSeenAt),
			row.ChatID,
		)
	}

	return strings.TrimSpace(text), userListKeyboard(rows, page, total), nil
}

func buildGroupList(pageValues ...string) (string, gotgbot.InlineKeyboardMarkup, error) {
	page := parseAdminPage(pageValues...)
	total, err := database.Q().CountChatsByType(context.Background(), database.ChatTypeGroup)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	page = clampAdminPage(page, total)

	rows, err := database.Q().ListChatsByTypePage(
		context.Background(),
		database.ListChatsByTypePageParams{
			Type:        database.ChatTypeGroup,
			LimitCount:  adminPageSize,
			OffsetCount: pageOffset(page),
		},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	if len(rows) == 0 {
		return "<b>👥 Gruplar</b>\n\nHenüz kayıtlı grup yok.", adminBackKeyboard(adminScreenHome), nil
	}

	text := fmt.Sprintf(
		"<b>👥 Gruplar</b>\n"+
			"Toplam: <b>%d</b> · Sayfa: <b>%d/%d</b>\n\n",
		total,
		page+1,
		totalAdminPages(total),
	)
	for index, row := range rows {
		text += fmt.Sprintf(
			"<b>%d.</b> %s\n%s\n\nID : <code>%d</code>\n\n",
			int(pageOffset(page))+index+1,
			formatAdminPageChatDisplayName(row),
			formatTimeAgo(row.LastSeenAt),
			row.ChatID,
		)
	}

	return strings.TrimSpace(text), groupListKeyboard(rows, page, total), nil
}

func buildMutedUserList() (string, gotgbot.InlineKeyboardMarkup, error) {
	total, err := database.Q().CountActiveMutedUsers(context.Background())
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	rows, err := database.Q().ListActiveMutedUsers(context.Background(), statsListLimit)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	if len(rows) == 0 {
		return "<b>🔇 Susturulan Kullanıcılar</b>\n\nAktif susturma yok.", mutedUserListKeyboard(rows), nil
	}

	text := fmt.Sprintf("<b>🔇 Susturulan Kullanıcılar</b>\nToplam: <b>%d</b>\n\n", total)
	for index, row := range rows {
		text += fmt.Sprintf(
			"<b>%d.</b> %s\n<code>%d</code> · kalan: %s\nSebep: %s\n\n",
			index+1,
			formatMutedUserDisplayName(row),
			row.UserID,
			formatDurationLeft(row.ExpiresAt.Time),
			html.EscapeString(row.Reason),
		)
	}

	return strings.TrimSpace(text), mutedUserListKeyboard(rows), nil
}

func buildBannedUserList() (string, gotgbot.InlineKeyboardMarkup, error) {
	total, err := database.Q().CountBannedUsers(context.Background())
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	rows, err := database.Q().ListBannedUsers(context.Background(), statsListLimit)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	if len(rows) == 0 {
		return "<b>⛔ Banlı Kullanıcılar</b>\n\nHenüz banlı kullanıcı yok.", bannedUserListKeyboard(rows), nil
	}

	text := fmt.Sprintf("<b>⛔ Banlı Kullanıcılar</b>\nToplam: <b>%d</b>\n\n", total)
	for index, row := range rows {
		text += fmt.Sprintf(
			"<b>%d.</b> %s\n<code>%d</code> · %s\nSebep: %s\n\n",
			index+1,
			formatBannedUserDisplayName(row),
			row.UserID,
			formatTimeAgo(row.CreatedAt),
			html.EscapeString(row.Reason),
		)
	}

	return strings.TrimSpace(text), bannedUserListKeyboard(rows), nil
}

func buildUserProfile(value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return buildUserList()
	}

	user, err := database.Q().GetChatByID(context.Background(), userID)
	if err != nil {
		return buildUnknownUserProfile(userID)
	}

	banned, err := database.Q().IsUserBanned(context.Background(), user.ChatID)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	muteExpiresAt, muted, err := getActiveMuteExpiresAt(user.ChatID)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	status := "Aktif"
	if banned {
		status = "Banlı"
	} else if muted {
		status = "Susturuldu · kalan: " + formatDurationLeft(muteExpiresAt)
	}

	summary, err := database.Q().GetUserDownloadSummary(context.Background(), user.ChatID)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	platforms, err := database.Q().ListUserPlatformStats(
		context.Background(),
		database.ListUserPlatformStatsParams{UserID: user.ChatID, LimitCount: adminActivityLimit},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	recentDownloads, err := database.Q().ListUserRecentDownloadEvents(
		context.Background(),
		database.ListUserRecentDownloadEventsParams{UserID: user.ChatID, LimitCount: adminActivityLimit},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	text := fmt.Sprintf(
		"<b>👤 Kullanıcı Profili</b>\n\n"+
			"%s\n"+
			"ID: <code>%d</code>\n"+
			"Kullanıcı adı: %s\n"+
			"Durum: %s\n"+
			"Kayıt: %s\n"+
			"Son görülme: %s\n\n"+
			"%s\n\n"+
			"%s\n\n"+
			"%s",
		formatUserProfileDisplayName(user),
		user.ChatID,
		formatUsername(user.Username),
		status,
		formatTimeAgo(user.CreatedAt),
		formatTimeAgo(user.LastSeenAt),
		formatDownloadActivitySummary(summary.Downloads, summary.Items, summary.TotalSize, summary.LastDownloadAt),
		formatUserPlatformBreakdown(platforms),
		formatUserRecentDownloadEvents(recentDownloads),
	)

	return text, userProfileKeyboard(user.ChatID, banned, muted), nil
}

func buildGroupProfile(value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	groupID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return buildGroupList()
	}

	group, err := database.Q().GetChatByID(context.Background(), groupID)
	if err != nil {
		return buildGroupList()
	}

	summary, err := database.Q().GetChatDownloadSummary(context.Background(), group.ChatID)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	platforms, err := database.Q().ListChatPlatformStats(
		context.Background(),
		database.ListChatPlatformStatsParams{ChatID: group.ChatID, LimitCount: adminActivityLimit},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	recentDownloads, err := database.Q().ListChatRecentDownloadEvents(
		context.Background(),
		database.ListChatRecentDownloadEventsParams{ChatID: group.ChatID, LimitCount: adminActivityLimit},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	text := fmt.Sprintf(
		"<b>👥 Grup Detayı</b>\n\n"+
			"%s\n"+
			"ID: <code>%d</code>\n"+
			"Kullanıcı adı: %s\n"+
			"Kayıt: %s\n"+
			"Son aktiflik: %s\n\n"+
			"%s\n\n"+
			"%s\n\n"+
			"%s",
		formatUserProfileDisplayName(group),
		group.ChatID,
		formatUsername(group.Username),
		formatTimeAgo(group.CreatedAt),
		formatTimeAgo(group.LastSeenAt),
		formatDownloadActivitySummary(summary.Downloads, summary.Items, summary.TotalSize, summary.LastDownloadAt),
		formatChatPlatformBreakdown(platforms),
		formatChatRecentDownloadEvents(recentDownloads),
	)

	return text, groupProfileKeyboard(), nil
}

func formatDownloadActivitySummary(downloads int64, items int64, totalSize int64, lastDownloadAt pgtype.Timestamptz) string {
	if downloads == 0 {
		return "<b>📈 Aktivite</b>\nHenüz indirme kaydı yok. Yeni indirmeler burada birikecek."
	}
	return fmt.Sprintf(
		"<b>📈 Aktivite</b>\n"+
			"İndirme: <b>%d</b> · Medya: <b>%d</b>\n"+
			"Toplam boyut: <b>%s</b>\n"+
			"Son indirme: <b>%s</b>",
		downloads,
		items,
		formatBytes(totalSize),
		formatTimeAgo(lastDownloadAt),
	)
}

func formatUserPlatformBreakdown(rows []database.ListUserPlatformStatsRow) string {
	if len(rows) == 0 {
		return "<b>🧩 Platformlar</b>\nKayıt yok."
	}
	lines := []string{"<b>🧩 Platformlar</b>"}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf(
			"%s · <b>%d</b> indirme · %s",
			html.EscapeString(row.ExtractorID),
			row.Downloads,
			formatBytes(row.TotalSize),
		))
	}
	return strings.Join(lines, "\n")
}

func formatChatPlatformBreakdown(rows []database.ListChatPlatformStatsRow) string {
	if len(rows) == 0 {
		return "<b>🧩 Platformlar</b>\nKayıt yok."
	}
	lines := []string{"<b>🧩 Platformlar</b>"}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf(
			"%s · <b>%d</b> indirme · %s",
			html.EscapeString(row.ExtractorID),
			row.Downloads,
			formatBytes(row.TotalSize),
		))
	}
	return strings.Join(lines, "\n")
}

func formatUserRecentDownloadEvents(rows []database.ListUserRecentDownloadEventsRow) string {
	if len(rows) == 0 {
		return "<b>🕘 Son İndirmeler</b>\nKayıt yok."
	}
	lines := []string{"<b>🕘 Son İndirmeler</b>"}
	for index, row := range rows {
		lines = append(lines, fmt.Sprintf(
			"%d. %s · %s%s · %d medya · %s · %s%s",
			index+1,
			formatDownloadEventLink(row.ContentUrl, row.ContentID),
			html.EscapeString(row.ExtractorID),
			formatCacheMarker(row.FromCache),
			row.ItemCount,
			formatBytes(row.TotalFileSize),
			formatTimeAgo(row.CreatedAt),
			formatEventChatSuffix(row.ChatType, row.ChatID, row.ChatTitle, row.ChatUsername),
		))
	}
	return strings.Join(lines, "\n")
}

func formatChatRecentDownloadEvents(rows []database.ListChatRecentDownloadEventsRow) string {
	if len(rows) == 0 {
		return "<b>🕘 Son İndirmeler</b>\nKayıt yok."
	}
	lines := []string{"<b>🕘 Son İndirmeler</b>"}
	for index, row := range rows {
		lines = append(lines, fmt.Sprintf(
			"%d. %s · %s%s · %d medya · %s · %s · %s",
			index+1,
			formatEventUserLabel(row.UserID, row.UserUsername, row.UserFirstName, row.UserLastName),
			html.EscapeString(row.ExtractorID),
			formatCacheMarker(row.FromCache),
			row.ItemCount,
			formatBytes(row.TotalFileSize),
			formatTimeAgo(row.CreatedAt),
			formatDownloadEventLink(row.ContentUrl, row.ContentID),
		))
	}
	return strings.Join(lines, "\n")
}

func formatDownloadEventLink(contentURL string, contentID string) string {
	label := strings.TrimSpace(contentID)
	if label == "" {
		label = "içerik"
	}
	label = truncateText(label, 28)
	if strings.TrimSpace(contentURL) == "" {
		return "<code>" + label + "</code>"
	}
	return fmt.Sprintf("<a href='%s'>%s</a>", html.EscapeString(contentURL), label)
}

func formatCacheMarker(fromCache bool) string {
	if !fromCache {
		return ""
	}
	return " · cache"
}

func formatEventChatSuffix(chatType database.ChatType, chatID int64, title pgtype.Text, username pgtype.Text) string {
	if chatType == database.ChatTypePrivate {
		return ""
	}
	name := validText(title)
	if name == "" && username.Valid && strings.TrimSpace(username.String) != "" {
		name = "@" + strings.TrimSpace(username.String)
	}
	if name == "" {
		name = strconv.FormatInt(chatID, 10)
	}
	return " · grup: " + html.EscapeString(name)
}

func formatEventUserLabel(userID int64, username pgtype.Text, firstName pgtype.Text, lastName pgtype.Text) string {
	name := strings.TrimSpace(joinValidTexts(firstName, lastName))
	if name == "" && username.Valid && strings.TrimSpace(username.String) != "" {
		name = "@" + strings.TrimSpace(username.String)
	}
	if name == "" {
		name = strconv.FormatInt(userID, 10)
	}
	return fmt.Sprintf(
		"<a href='tg://user?id=%d'>%s</a>",
		userID,
		html.EscapeString(name),
	)
}

func validText(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func buildUnknownUserProfile(userID int64) (string, gotgbot.InlineKeyboardMarkup, error) {
	banned, err := database.Q().IsUserBanned(context.Background(), userID)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	text := fmt.Sprintf(
		"<b>👤 Kullanıcı Profili</b>\n\n"+
			"ID: <code>%d</code>\n"+
			"Kullanıcı adı: bilinmiyor\n"+
			"Durum: %s\n\n"+
			"Bu kullanıcı henüz sohbet tablosunda kayıtlı değil.",
		userID,
		map[bool]string{true: "banlı", false: "bilinmiyor"}[banned],
	)
	_, muted, err := getActiveMuteExpiresAt(userID)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	return text, userProfileKeyboard(userID, banned, muted), nil
}

func buildBanConfirm(value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return buildUserList()
	}
	if util.IsAdminID(userID) {
		return "<b>🛡 Korumalı Kullanıcı</b>\n\nAdminler banlanamaz.", userProfileKeyboard(userID, false, false), nil
	}

	text := fmt.Sprintf(
		"<b>⛔ Ban Onayı</b>\n\n"+
			"Kullanıcı ID: <code>%d</code>\n\n"+
			"Kullanıcı özel sohbet, grup ve inline modda botu kullanamayacak.",
		userID,
	)
	return text, gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "⛔ Banı Onayla", CallbackData: adminCallbackPrefix + adminActionBan + ":" + strconv.FormatInt(userID, 10)},
			},
			{
				{Text: "⬅️ Geri", CallbackData: adminCallbackPrefix + adminScreenUser + ":" + strconv.FormatInt(userID, 10)},
			},
		},
	}, nil
}

func buildSystemPanel() (string, gotgbot.InlineKeyboardMarkup, error) {
	bannedUsersCount, err := database.Q().CountBannedUsers(context.Background())
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	mutedUsersCount, err := database.Q().CountActiveMutedUsers(context.Background())
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	text := fmt.Sprintf(
		"<b>🖥 Sistem</b>\n\n"+
			"Adminler: %d\n"+
			"Whitelist: %d\n"+
			"Banlı kullanıcı: %d\n"+
			"Susturulan kullanıcı: %d\n"+
			"Eşzamanlı işlem: %d\n"+
			"Maksimum süre: %s\n"+
			"Maksimum dosya: %s\n"+
			"Önbellek: %t\n"+
			"Log seviyesi: %s\n"+
			"Saat: %s",
		len(config.Env.Admins),
		len(config.Env.Whitelist),
		bannedUsersCount,
		mutedUsersCount,
		config.Env.ConcurrentUpdates,
		config.Env.MaxDuration,
		formatBytes(config.Env.MaxFileSize),
		config.Env.Caching,
		config.Env.LogLevel.String(),
		time.Now().Format("2006-01-02 15:04:05"),
	)

	return text, adminBackKeyboard(adminScreenHome), nil
}

func banUserFromCallback(ctx *ext.Context, value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return buildUserList()
	}
	if util.IsAdminID(userID) {
		return "<b>🛡 Korumalı Kullanıcı</b>\n\nAdminler banlanamaz.", userProfileKeyboard(userID, false, false), nil
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
	return buildUserProfile(value)
}

func unbanUserFromCallback(value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return buildUserList()
	}
	if err := database.Q().UnbanUser(context.Background(), userID); err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	return buildUserProfile(value)
}

func muteUserFromCallback(ctx *ext.Context, value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return buildUserList()
	}
	duration, err := parseCommandDuration(parts[0])
	if err != nil {
		return buildUserList()
	}
	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return buildUserList()
	}
	if util.IsAdminID(userID) {
		return "<b>🛡 Korumalı Kullanıcı</b>\n\nAdminler susturulamaz.", userProfileKeyboard(userID, false, false), nil
	}

	err = database.Q().MuteUser(
		context.Background(),
		database.MuteUserParams{
			UserID:    userID,
			Reason:    "admin panel",
			MutedBy:   ctx.CallbackQuery.From.Id,
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(duration), Valid: true},
		},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	return buildUserProfile(parts[1])
}

func unmuteUserFromCallback(value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return buildUserList()
	}
	if err := database.Q().UnmuteUser(context.Background(), userID); err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	return buildUserProfile(value)
}

func userListKeyboard(_ []database.ListChatsByTypePageRow, page int32, total int64) gotgbot.InlineKeyboardMarkup {
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, 4)
	buttons = append(buttons, adminPaginationRows(adminScreenUsers, page, total)...)
	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		{Text: "⛔ Banlılar", CallbackData: adminCallbackPrefix + adminScreenBans},
		{Text: "🔇 Susturulanlar", CallbackData: adminCallbackPrefix + adminScreenMutes},
	})
	buttons = append(buttons, adminBackRow(adminScreenHome))
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func groupListKeyboard(_ []database.ListChatsByTypePageRow, page int32, total int64) gotgbot.InlineKeyboardMarkup {
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, 2)
	buttons = append(buttons, adminPaginationRows(adminScreenGroups, page, total)...)
	buttons = append(buttons, adminBackRow(adminScreenHome))
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func bannedUserListKeyboard(_ []database.ListBannedUsersRow) gotgbot.InlineKeyboardMarkup {
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, 4)
	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		{Text: "👤 Kullanıcılar", CallbackData: adminCallbackPrefix + adminScreenUsers},
		{Text: "🔇 Susturulanlar", CallbackData: adminCallbackPrefix + adminScreenMutes},
	})
	buttons = append(buttons, adminBackRow(adminScreenModeration))
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func mutedUserListKeyboard(_ []database.ListActiveMutedUsersRow) gotgbot.InlineKeyboardMarkup {
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, 4)
	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		{Text: "👤 Kullanıcılar", CallbackData: adminCallbackPrefix + adminScreenUsers},
		{Text: "⛔ Banlılar", CallbackData: adminCallbackPrefix + adminScreenBans},
	})
	buttons = append(buttons, adminBackRow(adminScreenModeration))
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func userProfileKeyboard(userID int64, banned bool, muted bool) gotgbot.InlineKeyboardMarkup {
	actionText := "⛔ Banla"
	actionData := adminCallbackPrefix + adminActionBanConfirm + ":" + strconv.FormatInt(userID, 10)
	if banned {
		actionText = "✅ Banı Kaldır"
		actionData = adminCallbackPrefix + adminActionUnban + ":" + strconv.FormatInt(userID, 10)
	}

	muteRow := []gotgbot.InlineKeyboardButton{
		{Text: "🔇 1h Sustur", CallbackData: adminCallbackPrefix + adminActionMute + ":1h:" + strconv.FormatInt(userID, 10)},
		{Text: "🔇 24h Sustur", CallbackData: adminCallbackPrefix + adminActionMute + ":24h:" + strconv.FormatInt(userID, 10)},
	}
	if muted {
		muteRow = []gotgbot.InlineKeyboardButton{
			{Text: "🔊 Susturmayı Kaldır", CallbackData: adminCallbackPrefix + adminActionUnmute + ":" + strconv.FormatInt(userID, 10)},
		}
	}

	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: actionText, CallbackData: actionData},
			},
			muteRow,
			{
				{Text: "👤 Kullanıcılar", CallbackData: adminCallbackPrefix + adminScreenUsers},
				{Text: "⛔ Banlılar", CallbackData: adminCallbackPrefix + adminScreenBans},
			},
			adminBackRow(adminScreenModeration),
		},
	}
}

func groupProfileKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "👥 Gruplar", CallbackData: adminCallbackPrefix + adminScreenGroups},
				{Text: "📊 Analitik", CallbackData: statsCallbackPrefix + statsScreenSummary + ":" + statsPeriodAll},
			},
			adminBackRow(adminScreenHome),
		},
	}
}

func adminBackKeyboard(screen string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			adminBackRow(screen),
		},
	}
}

func adminPaginationRows(screen string, page int32, total int64) [][]gotgbot.InlineKeyboardButton {
	totalPages := totalAdminPages(total)
	if totalPages <= 1 {
		return nil
	}

	currentPage := strconv.FormatInt(int64(page), 10)
	row := make([]gotgbot.InlineKeyboardButton, 0, 3)
	if page > 0 {
		row = append(row, gotgbot.InlineKeyboardButton{
			Text:         "⬅️ Önceki",
			CallbackData: adminCallbackPrefix + screen + ":" + strconv.FormatInt(int64(page-1), 10),
		})
	} else {
		row = append(row, gotgbot.InlineKeyboardButton{
			Text:         "İlk sayfa",
			CallbackData: adminCallbackPrefix + screen + ":" + currentPage,
		})
	}
	row = append(row, gotgbot.InlineKeyboardButton{
		Text:         fmt.Sprintf("%d/%d", page+1, totalPages),
		CallbackData: adminCallbackPrefix + screen + ":" + currentPage,
	})
	if page+1 < totalPages {
		row = append(row, gotgbot.InlineKeyboardButton{
			Text:         "Sonraki ➡️",
			CallbackData: adminCallbackPrefix + screen + ":" + strconv.FormatInt(int64(page+1), 10),
		})
	} else {
		row = append(row, gotgbot.InlineKeyboardButton{
			Text:         "Son sayfa",
			CallbackData: adminCallbackPrefix + screen + ":" + currentPage,
		})
	}
	return [][]gotgbot.InlineKeyboardButton{row}
}

func parseAdminPage(values ...string) int32 {
	if len(values) == 0 {
		return 0
	}
	page, err := strconv.ParseInt(strings.TrimSpace(values[0]), 10, 32)
	if err != nil || page < 0 {
		return 0
	}
	return int32(page)
}

func clampAdminPage(page int32, total int64) int32 {
	totalPages := totalAdminPages(total)
	if totalPages == 0 {
		return 0
	}
	if page >= totalPages {
		return totalPages - 1
	}
	return page
}

func totalAdminPages(total int64) int32 {
	if total <= 0 {
		return 1
	}
	return int32((total + int64(adminPageSize) - 1) / int64(adminPageSize))
}

func pageOffset(page int32) int32 {
	return page * adminPageSize
}

func adminBackRow(screen string) []gotgbot.InlineKeyboardButton {
	return []gotgbot.InlineKeyboardButton{
		{Text: "⬅️ Geri", CallbackData: adminCallbackPrefix + screen},
	}
}

func formatBannedUserDisplayName(row database.ListBannedUsersRow) string {
	name := bannedUserDisplayLabel(row)
	return fmt.Sprintf(
		"<a href='tg://user?id=%d'>%s</a>",
		row.UserID,
		html.EscapeString(name),
	)
}

func formatMutedUserDisplayName(row database.ListActiveMutedUsersRow) string {
	name := bannedUserDisplayLabel(database.ListBannedUsersRow{
		UserID:    row.UserID,
		Username:  row.Username,
		FirstName: row.FirstName,
		LastName:  row.LastName,
	})
	return fmt.Sprintf(
		"<a href='tg://user?id=%d'>%s</a>",
		row.UserID,
		html.EscapeString(name),
	)
}

func formatAdminChatDisplayName(chat database.ListChatsByTypeRow) string {
	name := adminChatDisplayLabel(chat)
	result := "<b>" + html.EscapeString(name) + "</b>"
	username := strings.TrimSpace(chat.Username)
	if username != "" && !strings.Contains(strings.ToLower(name), strings.ToLower("@"+username)) {
		result += " @" + html.EscapeString(username)
	}
	return result
}

func formatAdminPageChatDisplayName(chat database.ListChatsByTypePageRow) string {
	name := adminPageChatDisplayLabel(chat)
	result := "<b>" + html.EscapeString(name) + "</b>"
	username := strings.TrimSpace(chat.Username)
	if username != "" && !strings.Contains(strings.ToLower(name), strings.ToLower("@"+username)) {
		result += " @" + html.EscapeString(username)
	}
	return result
}

func adminChatDisplayLabel(chat database.ListChatsByTypeRow) string {
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
	return name
}

func adminPageChatDisplayLabel(chat database.ListChatsByTypePageRow) string {
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
	return name
}

func getActiveMuteExpiresAt(userID int64) (time.Time, bool, error) {
	activeMute, err := database.Q().GetActiveMute(context.Background(), userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return activeMute.ExpiresAt.Time, true, nil
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

func formatUsername(username string) string {
	if strings.TrimSpace(username) == "" {
		return "-"
	}
	return "@" + html.EscapeString(username)
}

func formatUserProfileDisplayName(user database.GetChatByIDRow) string {
	name := strings.TrimSpace(user.Title)
	if name == "" {
		name = strings.TrimSpace(strings.Join([]string{user.FirstName, user.LastName}, " "))
	}
	if name == "" && user.Username != "" {
		name = "@" + user.Username
	}
	if name == "" {
		name = strconv.FormatInt(user.ChatID, 10)
	}
	if user.Type == database.ChatTypePrivate {
		return fmt.Sprintf(
			"<a href='tg://user?id=%d'>%s</a>",
			user.ChatID,
			html.EscapeString(name),
		)
	}
	return html.EscapeString(name)
}
