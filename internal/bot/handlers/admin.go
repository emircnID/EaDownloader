package handlers

import (
	"context"
	"errors"
	"fmt"
	"html"
	"runtime"
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

var StartTime = time.Now()

const (
	adminCallbackPrefix = "admin:"

	adminScreenHome       = "home"
	adminScreenSystem     = "system"
	adminScreenDbCleanup  = "db_cleanup"
	adminScreenUsers      = "users"
	adminScreenGroups     = "groups"
	adminScreenBans       = "bans"
	adminScreenMutes      = "mutes"
	adminScreenGroupBans  = "group_bans"
	adminScreenGroupMutes = "group_mutes"
	adminScreenUser       = "user"
	adminScreenGroup      = "group"

	adminActionBanConfirm = "ban_confirm"
	adminActionBan        = "ban"
	adminActionUnban      = "unban"
	adminActionMute       = "mute"
	adminActionUnmute     = "unmute"

	adminActionGroupBanConfirm = "group_ban_confirm"
	adminActionGroupBan        = "group_ban"
	adminActionGroupUnban      = "group_unban"
	adminActionGroupMute       = "group_mute"
	adminActionGroupUnmute     = "group_unmute"

	adminPageSize      int32 = 5
	adminActivityLimit int32 = 5

	statusActive = "Aktif"
	statusBanned = "Banlı"
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

	text, keyboard, err := resolveAdminCallback(bot, ctx)
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

func resolveAdminCallback(bot *gotgbot.Bot, ctx *ext.Context) (string, gotgbot.InlineKeyboardMarkup, error) {
	data := strings.TrimPrefix(ctx.CallbackQuery.Data, adminCallbackPrefix)

	switch {
	case data == adminScreenHome:
		return buildAdminHome()
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
	case data == adminScreenGroupBans:
		return buildBannedGroupList()
	case data == adminScreenGroupMutes:
		return buildMutedGroupList()
	case data == adminScreenSystem:
		return buildSystemPanel()
	case data == adminScreenDbCleanup:
		return buildDbCleanupPanel("")
	case strings.HasPrefix(data, "db_clean:"):
		return handleDbCleanup(bot, ctx, strings.TrimPrefix(data, "db_clean:"))
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
	case strings.HasPrefix(data, adminActionGroupBanConfirm+":"):
		return buildGroupBanConfirm(strings.TrimPrefix(data, adminActionGroupBanConfirm+":"))
	case strings.HasPrefix(data, adminActionGroupBan+":"):
		return banGroupFromCallback(ctx, strings.TrimPrefix(data, adminActionGroupBan+":"))
	case strings.HasPrefix(data, adminActionGroupUnban+":"):
		return unbanGroupFromCallback(strings.TrimPrefix(data, adminActionGroupUnban+":"))
	case strings.HasPrefix(data, adminActionGroupMute+":"):
		return muteGroupFromCallback(ctx, strings.TrimPrefix(data, adminActionGroupMute+":"))
	case strings.HasPrefix(data, adminActionGroupUnmute+":"):
		return unmuteGroupFromCallback(strings.TrimPrefix(data, adminActionGroupUnmute+":"))
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
	bannedUsersCount, err := database.Q().CountBannedChatsByType(context.Background(), database.ChatTypePrivate)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	mutedUsersCount, err := database.Q().CountActiveMutedChatsByType(context.Background(), database.ChatTypePrivate)
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
				{Text: "🖥 Sistem Paneli", CallbackData: adminCallbackPrefix + adminScreenSystem},
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
		status := statusActive
		if banned, err := database.Q().IsUserBanned(context.Background(), row.ChatID); err == nil && banned {
			status = statusBanned
		} else if activeMute, err := database.Q().GetActiveMute(context.Background(), row.ChatID); err == nil {
			status = "Susturuldu: " + formatDurationLeft(activeMute.ExpiresAt.Time)
		}
		text += fmt.Sprintf(
			"<b>%d.</b> %s\n%s · %s\nID : <code>%d</code>\n\n",
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
		status := statusActive
		if banned, err := database.Q().IsUserBanned(context.Background(), row.ChatID); err == nil && banned {
			status = statusBanned
		} else if activeMute, err := database.Q().GetActiveMute(context.Background(), row.ChatID); err == nil {
			status = "Susturuldu: " + formatDurationLeft(activeMute.ExpiresAt.Time)
		}
		text += fmt.Sprintf(
			"<b>%d.</b> %s\n%s · %s\nID : <code>%d</code>\n\n",
			int(pageOffset(page))+index+1,
			formatAdminPageChatDisplayName(row),
			status,
			formatTimeAgo(row.LastSeenAt),
			row.ChatID,
		)
	}

	return strings.TrimSpace(text), groupListKeyboard(rows, page, total), nil
}

func buildMutedGroupList() (string, gotgbot.InlineKeyboardMarkup, error) {
	total, err := database.Q().CountActiveMutedChatsByType(context.Background(), database.ChatTypeGroup)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	rows, err := database.Q().ListActiveMutedChatsByType(
		context.Background(),
		database.ListActiveMutedChatsByTypeParams{Type: database.ChatTypeGroup, LimitCount: statsListLimit},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	if len(rows) == 0 {
		return "<b>🔇 Susturulan Gruplar</b>\n\nAktif susturulan grup yok.", mutedGroupListKeyboard(), nil
	}

	text := fmt.Sprintf("<b>🔇 Susturulan Gruplar</b>\nToplam: <b>%d</b>\n\n", total)
	for index, row := range rows {
		text += fmt.Sprintf(
			"<b>%d.</b> %s\n<code>%d</code> · kalan: %s\nSebep: %s\n\n",
			index+1,
			formatBannedChatDisplayName(row.UserID, row.Title, row.Username, row.FirstName, row.LastName),
			row.UserID,
			formatDurationLeft(row.ExpiresAt.Time),
			html.EscapeString(row.Reason),
		)
	}

	return strings.TrimSpace(text), mutedGroupListKeyboard(), nil
}

func buildBannedGroupList() (string, gotgbot.InlineKeyboardMarkup, error) {
	total, err := database.Q().CountBannedChatsByType(context.Background(), database.ChatTypeGroup)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	rows, err := database.Q().ListBannedChatsByType(
		context.Background(),
		database.ListBannedChatsByTypeParams{Type: database.ChatTypeGroup, LimitCount: statsListLimit},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	if len(rows) == 0 {
		return "<b>⛔ Banlı Gruplar</b>\n\nHenüz banlı grup yok.", bannedGroupListKeyboard(), nil
	}

	text := fmt.Sprintf("<b>⛔ Banlı Gruplar</b>\nToplam: <b>%d</b>\n\n", total)
	for index, row := range rows {
		text += fmt.Sprintf(
			"<b>%d.</b> %s\n<code>%d</code> · %s\nSebep: %s\n\n",
			index+1,
			formatBannedChatDisplayName(row.UserID, row.Title, row.Username, row.FirstName, row.LastName),
			row.UserID,
			formatTimeAgo(row.CreatedAt),
			html.EscapeString(row.Reason),
		)
	}

	return strings.TrimSpace(text), bannedGroupListKeyboard(), nil
}

func buildMutedUserList() (string, gotgbot.InlineKeyboardMarkup, error) {
	total, err := database.Q().CountActiveMutedChatsByType(context.Background(), database.ChatTypePrivate)
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
	total, err := database.Q().CountBannedChatsByType(context.Background(), database.ChatTypePrivate)
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

	status := statusActive
	if banned {
		status = statusBanned
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

	banned, err := database.Q().IsUserBanned(context.Background(), group.ChatID)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	muteExpiresAt, muted, err := getActiveMuteExpiresAt(group.ChatID)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	status := statusActive
	if banned {
		status = statusBanned
	} else if muted {
		status = "Susturuldu · kalan: " + formatDurationLeft(muteExpiresAt)
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
			"Durum: %s\n"+
			"Kayıt: %s\n"+
			"Son aktiflik: %s\n\n"+
			"%s\n\n"+
			"%s\n\n"+
			"%s",
		formatUserProfileDisplayName(group),
		group.ChatID,
		formatUsername(group.Username),
		status,
		formatTimeAgo(group.CreatedAt),
		formatTimeAgo(group.LastSeenAt),
		formatDownloadActivitySummary(summary.Downloads, summary.Items, summary.TotalSize, summary.LastDownloadAt),
		formatChatPlatformBreakdown(platforms),
		formatChatRecentDownloadEvents(recentDownloads),
	)

	return text, groupProfileKeyboard(group.ChatID, banned, muted), nil
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
				{Text: "👤 Profil", CallbackData: adminCallbackPrefix + adminScreenUser + ":" + strconv.FormatInt(userID, 10)},
			},
		},
	}, nil
}

func buildGroupBanConfirm(value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	groupID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return buildGroupList()
	}

	text := fmt.Sprintf(
		"<b>⛔ Grup Ban Onayı</b>\n\n"+
			"Grup ID: <code>%d</code>\n\n"+
			"Bu grupta bot komutları ve link indirme işlemleri engellenecek.",
		groupID,
	)
	return text, gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "⛔ Grubu Banla", CallbackData: adminCallbackPrefix + adminActionGroupBan + ":" + strconv.FormatInt(groupID, 10)},
			},
			{
				{Text: "👥 Grup Profili", CallbackData: adminCallbackPrefix + adminScreenGroup + ":" + strconv.FormatInt(groupID, 10)},
			},
			adminHomeRow(),
		},
	}, nil
}

func formatUptime(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d %= time.Hour
	m := d / time.Minute
	d %= time.Minute
	s := d / time.Second
	return fmt.Sprintf("%d sa %d dk %d sn", h, m, s)
}

func buildSystemPanel() (string, gotgbot.InlineKeyboardMarkup, error) {
	bannedUsersCount, err := database.Q().CountBannedChatsByType(context.Background(), database.ChatTypePrivate)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	mutedUsersCount, err := database.Q().CountActiveMutedChatsByType(context.Background(), database.ChatTypePrivate)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := time.Since(StartTime)

	text := fmt.Sprintf(
		"<b>🖥 Sistem Durumu</b>\n\n"+
			"⏱ Çalışma Süresi: <b>%s</b>\n"+
			"🧵 Aktif Goroutine: <b>%d</b>\n"+
			"🐏 Kullanılan Bellek: <b>%.2f MB</b>\n"+
			"💾 Sistem Belleği: <b>%.2f MB</b>\n"+
			"⚙️ CPU Çekirdeği: <b>%d</b>\n\n"+
			"<b>⚙️ Konfigürasyon</b>\n"+
			"Adminler: %d\n"+
			"Whitelist: %d\n"+
			"Banlı chat: %d\n"+
			"Susturulan chat: %d\n"+
			"Eşzamanlı işlem: %d\n"+
			"Maksimum süre: %s\n"+
			"Maksimum dosya: %s\n"+
			"Önbellek: %t\n"+
			"Log seviyesi: %s\n"+
			"Saat: %s",
		formatUptime(uptime),
		runtime.NumGoroutine(),
		float64(mem.Alloc)/1024/1024,
		float64(mem.Sys)/1024/1024,
		runtime.NumCPU(),
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

	return text, systemPanelKeyboard(), nil
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

func banGroupFromCallback(ctx *ext.Context, value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	groupID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return buildGroupList()
	}

	_, err = database.Q().BanUser(
		context.Background(),
		database.BanUserParams{
			UserID:   groupID,
			Reason:   "admin panel group",
			BannedBy: ctx.CallbackQuery.From.Id,
		},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	return buildGroupProfile(value)
}

func unbanGroupFromCallback(value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	groupID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return buildGroupList()
	}
	if err := database.Q().UnbanUser(context.Background(), groupID); err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	return buildGroupProfile(value)
}

func muteGroupFromCallback(ctx *ext.Context, value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return buildGroupList()
	}
	duration, err := parseCommandDuration(parts[0])
	if err != nil {
		return buildGroupList()
	}
	groupID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return buildGroupList()
	}

	err = database.Q().MuteUser(
		context.Background(),
		database.MuteUserParams{
			UserID:    groupID,
			Reason:    "admin panel group",
			MutedBy:   ctx.CallbackQuery.From.Id,
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(duration), Valid: true},
		},
	)
	if err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	return buildGroupProfile(parts[1])
}

func unmuteGroupFromCallback(value string) (string, gotgbot.InlineKeyboardMarkup, error) {
	groupID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return buildGroupList()
	}
	if err := database.Q().UnmuteUser(context.Background(), groupID); err != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, err
	}
	return buildGroupProfile(value)
}

func userListKeyboard(_ []database.ListChatsByTypePageRow, page int32, total int64) gotgbot.InlineKeyboardMarkup {
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, 4)
	buttons = append(buttons, adminPaginationRows(adminScreenUsers, page, total)...)
	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		{Text: "⛔ Banlılar", CallbackData: adminCallbackPrefix + adminScreenBans},
		{Text: "🔇 Susturulanlar", CallbackData: adminCallbackPrefix + adminScreenMutes},
	})
	buttons = append(buttons, adminHomeRow())
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func groupListKeyboard(_ []database.ListChatsByTypePageRow, page int32, total int64) gotgbot.InlineKeyboardMarkup {
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, 3)
	buttons = append(buttons, adminPaginationRows(adminScreenGroups, page, total)...)
	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		{Text: "⛔ Banlı Gruplar", CallbackData: adminCallbackPrefix + adminScreenGroupBans},
		{Text: "🔇 Susturulan Gruplar", CallbackData: adminCallbackPrefix + adminScreenGroupMutes},
	})
	buttons = append(buttons, adminHomeRow())
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func bannedGroupListKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "👥 Gruplar", CallbackData: adminCallbackPrefix + adminScreenGroups},
				{Text: "🔇 Susturulan Gruplar", CallbackData: adminCallbackPrefix + adminScreenGroupMutes},
			},
			adminHomeRow(),
		},
	}
}

func mutedGroupListKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "👥 Gruplar", CallbackData: adminCallbackPrefix + adminScreenGroups},
				{Text: "⛔ Banlı Gruplar", CallbackData: adminCallbackPrefix + adminScreenGroupBans},
			},
			adminHomeRow(),
		},
	}
}

func systemPanelKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "🚨 Hatalar", CallbackData: statsCallbackPrefix + statsScreenErrors},
				{Text: "🧹 Veritabanı Temizliği", CallbackData: adminCallbackPrefix + adminScreenDbCleanup},
			},
			adminHomeRow(),
		},
	}
}

func buildDbCleanupPanel(statusMessage string) (string, gotgbot.InlineKeyboardMarkup, error) {
	adminIDs := []string{}
	for _, adminID := range config.Env.Admins {
		adminIDs = append(adminIDs, strconv.FormatInt(adminID, 10))
	}
	excludeAdmins := strings.Join(adminIDs, ",")
	if excludeAdmins == "" {
		excludeAdmins = "0"
	}

	var countUsers int64
	err := database.Conn().QueryRow(context.Background(), fmt.Sprintf(`
		SELECT COUNT(*) FROM chat 
		WHERE type = 'private' 
		  AND chat_id NOT IN (SELECT chat_id FROM download_events WHERE chat_type = 'private')
		  AND chat_id NOT IN (SELECT user_id FROM banned_users)
		  AND chat_id NOT IN (SELECT user_id FROM muted_users)
		  AND chat_id NOT IN (%s)
	`, excludeAdmins)).Scan(&countUsers)
	if err != nil {
		countUsers = 0
	}

	var countGroups int64
	err = database.Conn().QueryRow(context.Background(), `
		SELECT COUNT(*) FROM chat 
		WHERE type = 'group' 
		  AND chat_id NOT IN (SELECT chat_id FROM download_events WHERE chat_type = 'group')
		  AND chat_id NOT IN (SELECT user_id FROM banned_users)
		  AND chat_id NOT IN (SELECT user_id FROM muted_users)
	`).Scan(&countGroups)
	if err != nil {
		countGroups = 0
	}

	var countDownloads int64
	err = database.Conn().QueryRow(context.Background(), "SELECT COUNT(*) FROM download_events").Scan(&countDownloads)
	if err != nil {
		countDownloads = 0
	}

	var countErrors int64
	err = database.Conn().QueryRow(context.Background(), "SELECT COUNT(*) FROM errors").Scan(&countErrors)
	if err != nil {
		countErrors = 0
	}

	text := "<b>🧹 Veritabanı Temizliği</b>\n\n"
	if statusMessage != "" {
		text += statusMessage + "\n\n"
	}
	text += fmt.Sprintf(
		"Temizlenebilecek veriler:\n"+
			"👤 İnaktif kullanıcılar: <b>%d</b>\n"+
			"👥 İnaktif gruplar: <b>%d</b>\n"+
			"📥 İndirme geçmişi: <b>%d</b> kayıt\n"+
			"🚨 Hata kayıtları: <b>%d</b> kayıt\n\n"+
			"Temizlemek istediğiniz kategoriyi seçin.",
		countUsers,
		countGroups,
		countDownloads,
		countErrors,
	)

	keyboard := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "👤 İnaktif Kullanıcıları Temizle", CallbackData: adminCallbackPrefix + "db_clean:users"},
			},
			{
				{Text: "👥 İnaktif Grupları Temizle", CallbackData: adminCallbackPrefix + "db_clean:groups"},
			},
			{
				{Text: "📥 İndirme Geçmişini Temizle", CallbackData: adminCallbackPrefix + "db_clean:downloads"},
				{Text: "🚨 Hataları Temizle", CallbackData: adminCallbackPrefix + "db_clean:errors"},
			},
			{
				{Text: "⬅️ Sistem Paneli", CallbackData: adminCallbackPrefix + adminScreenSystem},
				{Text: "🏠 Anamenü", CallbackData: adminCallbackPrefix + adminScreenHome},
			},
		},
	}

	return text, keyboard, nil
}

func handleDbCleanup(bot *gotgbot.Bot, ctx *ext.Context, target string) (string, gotgbot.InlineKeyboardMarkup, error) {
	ctx.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{
		Text: "Temizleniyor...",
	})

	var status string
	switch target {
	case "users":
		adminIDs := []string{}
		for _, adminID := range config.Env.Admins {
			adminIDs = append(adminIDs, strconv.FormatInt(adminID, 10))
		}
		excludeAdmins := strings.Join(adminIDs, ",")
		if excludeAdmins == "" {
			excludeAdmins = "0"
		}

		query := fmt.Sprintf(`
			DELETE FROM chat 
			WHERE type = 'private' 
			  AND chat_id NOT IN (SELECT chat_id FROM download_events WHERE chat_type = 'private')
			  AND chat_id NOT IN (SELECT user_id FROM banned_users)
			  AND chat_id NOT IN (SELECT user_id FROM muted_users)
			  AND chat_id NOT IN (%s)
		`, excludeAdmins)

		tag, err := database.Conn().Exec(context.Background(), query)
		if err != nil {
			status = "❌ Hata: " + err.Error()
		} else {
			status = fmt.Sprintf("✅ <b>%d</b> inaktif kullanıcı veritabanından temizlendi.", tag.RowsAffected())
		}

	case "groups":
		query := `
			DELETE FROM chat 
			WHERE type = 'group' 
			  AND chat_id NOT IN (SELECT chat_id FROM download_events WHERE chat_type = 'group')
			  AND chat_id NOT IN (SELECT user_id FROM banned_users)
			  AND chat_id NOT IN (SELECT user_id FROM muted_users)
		`

		tag, err := database.Conn().Exec(context.Background(), query)
		if err != nil {
			status = "❌ Hata: " + err.Error()
		} else {
			status = fmt.Sprintf("✅ <b>%d</b> inaktif grup veritabanından temizlendi.", tag.RowsAffected())
		}

	case "downloads":
		_, err := database.Conn().Exec(context.Background(), "TRUNCATE TABLE download_events CASCADE")
		if err != nil {
			status = "❌ Hata: " + err.Error()
		} else {
			status = "✅ İndirme geçmişi tamamen temizlendi."
		}

	case "errors":
		_, err := database.Conn().Exec(context.Background(), "TRUNCATE TABLE errors")
		if err != nil {
			status = "❌ Hata: " + err.Error()
		} else {
			status = "✅ Hata kayıtları temizlendi."
		}
	}

	return buildDbCleanupPanel(status)
}

func bannedUserListKeyboard(_ []database.ListBannedUsersRow) gotgbot.InlineKeyboardMarkup {
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, 4)
	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		{Text: "👤 Kullanıcılar", CallbackData: adminCallbackPrefix + adminScreenUsers},
		{Text: "🔇 Susturulanlar", CallbackData: adminCallbackPrefix + adminScreenMutes},
	})
	buttons = append(buttons, adminHomeRow())
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func mutedUserListKeyboard(_ []database.ListActiveMutedUsersRow) gotgbot.InlineKeyboardMarkup {
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, 4)
	buttons = append(buttons, []gotgbot.InlineKeyboardButton{
		{Text: "👤 Kullanıcılar", CallbackData: adminCallbackPrefix + adminScreenUsers},
		{Text: "⛔ Banlılar", CallbackData: adminCallbackPrefix + adminScreenBans},
	})
	buttons = append(buttons, adminHomeRow())
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
			adminHomeRow(),
		},
	}
}

func groupProfileKeyboard(groupID int64, banned bool, muted bool) gotgbot.InlineKeyboardMarkup {
	actionText := "⛔ Grubu Banla"
	actionData := adminCallbackPrefix + adminActionGroupBanConfirm + ":" + strconv.FormatInt(groupID, 10)
	if banned {
		actionText = "✅ Grup Banını Kaldır"
		actionData = adminCallbackPrefix + adminActionGroupUnban + ":" + strconv.FormatInt(groupID, 10)
	}

	muteRow := []gotgbot.InlineKeyboardButton{
		{Text: "🔇 1h Sustur", CallbackData: adminCallbackPrefix + adminActionGroupMute + ":1h:" + strconv.FormatInt(groupID, 10)},
		{Text: "🔇 24h Sustur", CallbackData: adminCallbackPrefix + adminActionGroupMute + ":24h:" + strconv.FormatInt(groupID, 10)},
	}
	if muted {
		muteRow = []gotgbot.InlineKeyboardButton{
			{Text: "🔊 Susturmayı Kaldır", CallbackData: adminCallbackPrefix + adminActionGroupUnmute + ":" + strconv.FormatInt(groupID, 10)},
		}
	}

	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: actionText, CallbackData: actionData},
			},
			muteRow,
			{
				{Text: "👥 Gruplar", CallbackData: adminCallbackPrefix + adminScreenGroups},
				{Text: "📊 Analitik", CallbackData: statsCallbackPrefix + statsScreenSummary + ":" + statsPeriodAll},
			},
			adminHomeRow(),
		},
	}
}

func adminBackKeyboard(_ string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			adminHomeRow(),
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

func adminHomeRow() []gotgbot.InlineKeyboardButton {
	return []gotgbot.InlineKeyboardButton{
		{Text: "🏠 Anamenü", CallbackData: adminCallbackPrefix + adminScreenHome},
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

func formatBannedChatDisplayName(chatID int64, title string, username string, firstName string, lastName string) string {
	name := strings.TrimSpace(title)
	if name == "" {
		name = strings.TrimSpace(strings.Join([]string{firstName, lastName}, " "))
	}
	if name == "" && strings.TrimSpace(username) != "" {
		name = "@" + strings.TrimSpace(username)
	}
	if name == "" {
		name = strconv.FormatInt(chatID, 10)
	}

	result := "<b>" + html.EscapeString(normalizeDisplayLabel(name)) + "</b>"
	if username = strings.TrimSpace(username); username != "" && !strings.Contains(strings.ToLower(name), strings.ToLower("@"+username)) {
		result += " @" + html.EscapeString(username)
	}
	return result
}

func formatAdminChatDisplayName(chat database.ListChatsByTypeRow) string {
	name := normalizeDisplayLabel(adminChatDisplayLabel(chat))
	result := "<b>" + html.EscapeString(name) + "</b>"
	username := strings.TrimSpace(chat.Username)
	if username != "" && !strings.Contains(strings.ToLower(name), strings.ToLower("@"+username)) {
		result += " @" + html.EscapeString(username)
	}
	return result
}

func formatAdminPageChatDisplayName(chat database.ListChatsByTypePageRow) string {
	name := normalizeDisplayLabel(adminPageChatDisplayLabel(chat))
	result := "<b>" + html.EscapeString(name) + "</b>"
	username := strings.TrimSpace(chat.Username)
	if username != "" && !strings.Contains(strings.ToLower(name), strings.ToLower("@"+username)) {
		result += " @" + html.EscapeString(username)
	}
	return result
}

func normalizeDisplayLabel(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
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
	name := strings.TrimSpace(strings.Join([]string{row.FirstName, row.LastName}, " "))
	if name == "" && strings.TrimSpace(row.Username) != "" {
		name = "@" + strings.TrimSpace(row.Username)
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
