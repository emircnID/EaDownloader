package handlers

import (
	"fmt"
	"strings"
	"time"

	"eadownloader/internal/core"
	"eadownloader/internal/extractors/youtube"
	"eadownloader/internal/models"
	"eadownloader/internal/util"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

type youtubeTask struct {
	ExtractorCtx *models.ExtractorContext
}

var youtubeTasks = expirable.NewLRU[string, *youtubeTask](0, nil, 10*time.Minute)

func YouTubePromptHandler(bot *gotgbot.Bot, ctx *ext.Context, extractorCtx *models.ExtractorContext) error {
	taskID := uuid.NewString()[:8]
	if ok := youtubeTasks.Add(taskID, &youtubeTask{
		ExtractorCtx: extractorCtx,
	}); ok {
		extractorCtx.CancelFunc()
		return fmt.Errorf("failed to add youtube task")
	}

	_, err := ctx.EffectiveMessage.Reply(
		bot,
		"YouTube formatini sec:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: &gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: buildYouTubeButtons(taskID, youtube.SelectableFormatIDs()),
			},
		},
	)
	if err != nil {
		youtubeTasks.Remove(taskID)
		extractorCtx.CancelFunc()
		return err
	}

	return nil
}

func YouTubeShortsHandler(bot *gotgbot.Bot, ctx *ext.Context, extractorCtx *models.ExtractorContext) error {
	if err := util.SendTypingAction(bot, extractorCtx.Chat.ChatID); err != nil {
		return err
	}

	media, err := youtube.GetMedia(extractorCtx)
	if err != nil {
		return err
	}

	selectedMedia, err := youtube.SelectBestVideoMedia(media)
	if err != nil {
		return err
	}

	message := ctx.EffectiveMessage
	isSpoiler := util.HasHashtagEntity(message, "spoiler") ||
		util.HasHashtagEntity(message, "nsfw")

	return core.HandlePreparedDownloadTask(
		bot,
		ctx,
		extractorCtx,
		selectedMedia,
		isSpoiler,
		true,
	)
}

func YouTubeCallbackHandler(bot *gotgbot.Bot, ctx *ext.Context) error {
	data := ctx.CallbackQuery.Data
	parts := strings.Split(data, ".")
	if len(parts) != 3 {
		ctx.CallbackQuery.Answer(bot, nil)
		return ext.EndGroups
	}

	taskID := parts[1]
	formatID := parts[2]

	task, ok := youtubeTasks.Get(taskID)
	if !ok {
		ctx.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "Bu secim zaman asimina ugradi. Linki tekrar gonder.",
			ShowAlert: true,
		})
		return ext.EndGroups
	}
	youtubeTasks.Remove(taskID)
	defer task.ExtractorCtx.CancelFunc()
	defer task.ExtractorCtx.FilesTracker.Cleanup()

	ctx.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{
		Text: "Format bilgisi aliniyor...",
	})
	progress := youtubeProgressReporter(bot, ctx)
	task.ExtractorCtx.ProgressFunc = progress
	progress("Format bilgisi aliniyor...")

	media, err := youtube.GetMedia(task.ExtractorCtx)
	if err != nil {
		core.HandleError(bot, ctx, task.ExtractorCtx, err)
		return ext.EndGroups
	}

	selectedMedia, err := youtube.SelectMedia(media, formatID)
	if err != nil {
		core.HandleError(bot, ctx, task.ExtractorCtx, err)
		return ext.EndGroups
	}

	progress("Indirme hazirlaniyor...")

	if err := core.HandlePreparedDownloadTask(
		bot,
		ctx,
		task.ExtractorCtx,
		selectedMedia,
		false,
		true,
	); err != nil {
		core.HandleError(bot, ctx, task.ExtractorCtx, err)
		return ext.EndGroups
	}

	ctx.EffectiveMessage.Delete(bot, nil)
	return ext.EndGroups
}

func youtubeProgressReporter(bot *gotgbot.Bot, ctx *ext.Context) func(string) {
	var lastMessage string
	return func(message string) {
		if message == "" || message == lastMessage {
			return
		}
		lastMessage = message
		ctx.EffectiveMessage.EditText(
			bot,
			message,
			&gotgbot.EditMessageTextOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{},
			},
		)
	}
}

func buildYouTubeButtons(taskID string, formatIDs []string) [][]gotgbot.InlineKeyboardButton {
	var videoRow []gotgbot.InlineKeyboardButton
	var audioRow []gotgbot.InlineKeyboardButton

	for _, formatID := range formatIDs {
		button := gotgbot.InlineKeyboardButton{
			Text:         formatLabel(formatID),
			CallbackData: "youtube." + taskID + "." + formatID,
		}
		if formatID == "mp3" {
			audioRow = append(audioRow, button)
		} else {
			videoRow = append(videoRow, button)
		}
	}

	buttons := make([][]gotgbot.InlineKeyboardButton, 0, 2)
	if len(videoRow) > 0 {
		buttons = append(buttons, videoRow)
	}
	if len(audioRow) > 0 {
		buttons = append(buttons, audioRow)
	}
	return buttons
}

func formatLabel(formatID string) string {
	if formatID == "mp3" {
		return "MP3"
	}
	return formatID + "p"
}
