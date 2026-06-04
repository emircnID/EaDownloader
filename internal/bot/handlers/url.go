package handlers

import (
	"slices"

	"eadownloader/internal/core"
	"eadownloader/internal/extractors"
	"eadownloader/internal/extractors/youtube"
	"eadownloader/internal/logger"
	"eadownloader/internal/util"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

func URLFilter(msg *gotgbot.Message) bool {
	return message.Text(msg) &&
		!message.Command(msg) &&
		message.Entity("url")(msg)
}

func URLHandler(bot *gotgbot.Bot, ctx *ext.Context) error {
	message := ctx.EffectiveMessage

	url := util.URLFromMessage(message)
	if url == "" {
		return ext.EndGroups
	}

	if util.HasHashtagEntity(message, "skip") {
		return ext.EndGroups
	}

	extractorCtx := extractors.FromURL(url)
	if extractorCtx == nil || extractorCtx.Extractor == nil {
		return ext.EndGroups
	}

	chat, err := util.ChatFromContext(ctx)
	if err != nil {
		logger.L.Errorf("failed to get settings from context: %v", err)
		extractorCtx.CancelFunc()
		return ext.EndGroups
	}
	if chat != nil && slices.Contains(chat.DisabledExtractors, extractorCtx.Extractor.ID) {
		extractorCtx.CancelFunc()
		return ext.EndGroups
	}
	extractorCtx.SetChat(chat)

	if extractorCtx.Extractor.ID == "youtube" {
		if youtube.IsShortsURL(url) {
			defer extractorCtx.CancelFunc()
			defer extractorCtx.FilesTracker.Cleanup()

			err = YouTubeShortsHandler(bot, ctx, extractorCtx)
			if err != nil {
				core.HandleError(bot, ctx, extractorCtx, err)
			}
			return ext.EndGroups
		}

		err = YouTubePromptHandler(bot, ctx, extractorCtx)
		if err != nil {
			core.HandleError(bot, ctx, extractorCtx, err)
			return ext.EndGroups
		}
		return ext.EndGroups
	}

	defer extractorCtx.CancelFunc()

	err = util.SendTypingAction(bot, chat.ChatID)
	if err != nil {
		core.HandleError(bot, ctx, extractorCtx, err)
		return ext.EndGroups
	}

	err = core.HandleDownloadTask(bot, ctx, extractorCtx)
	if err != nil {
		core.HandleError(bot, ctx, extractorCtx, err)
		return ext.EndGroups
	}

	return ext.EndGroups
}
