package core

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"eadownloader/internal/config"
	"eadownloader/internal/database"
	"eadownloader/internal/models"
	"eadownloader/internal/util"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func SendFormats(
	bot *gotgbot.Bot,
	ctx *ext.Context,
	extractorCtx *models.ExtractorContext,
	media *models.Media,
	formats []*models.DownloadedFormat,
	options *models.SendFormatsOptions,
) ([]gotgbot.Message, error) {
	var chatID int64
	var messageOptions *gotgbot.SendMediaGroupOpts

	chat := extractorCtx.Chat

	if chat.Type == database.ChatTypeGroup {
		if len(formats) > int(chat.MediaAlbumLimit) {
			return nil, util.ErrMediaAlbumLimitExceeded
		}
		if !chat.Nsfw && media.NSFW {
			return nil, util.ErrNSFWNotAllowed
		}
	}

	switch {
	case ctx.Message != nil:
		chatID = ctx.EffectiveMessage.Chat.Id
		messageOptions = &gotgbot.SendMediaGroupOpts{
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                ctx.EffectiveMessage.MessageId,
				AllowSendingWithoutReply: true,
			},
		}
	case ctx.CallbackQuery != nil:
		chatID = ctx.CallbackQuery.Message.GetChat().Id
	case ctx.InlineQuery != nil:
		chatID = ctx.InlineQuery.From.Id
	case ctx.ChosenInlineResult != nil:
		chatID = ctx.ChosenInlineResult.From.Id
		messageOptions = &gotgbot.SendMediaGroupOpts{
			DisableNotification: true,
		}
	default:
		return nil, fmt.Errorf("failed to get chat id")
	}

	var sentMessages []gotgbot.Message

	mediaGroupChunks, err := chunkFormatsForUpload(formats)
	if err != nil {
		return nil, err
	}

	for _, chunk := range mediaGroupChunks {
		extractorCtx.Progress(uploadProgressMessage(chunk))
		if len(chunk) == 1 {
			util.SendMediaAction(bot, chatID, chunk[0].Format.Type)
			msg, err := sendSingleFormat(
				bot, chatID,
				chunk[0],
				options.Caption,
				options.IsSpoiler,
				messageOptions,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to send media: %w", err)
			}
			if options.Delete {
				go msg.Delete(bot, nil)
			}
			sentMessages = append(sentMessages, *msg)
			continue
		}

		var inputMediaList []gotgbot.InputMedia
		for i, f := range chunk {
			var caption string
			if i == 0 {
				caption = options.Caption
			}
			inputMedia, err := f.Format.GetInputMedia(
				f.FilePath, f.ThumbnailFilePath,
				caption, options.IsSpoiler,
				useLocalFilePathUpload(),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to get input media: %w", err)
			}
			inputMediaList = append(inputMediaList, inputMedia)
		}

		util.SendMediaAction(bot, chatID, chunk[0].Format.Type)

		msgs, err := bot.SendMediaGroup(
			chatID,
			inputMediaList,
			messageOptions,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to send media group: %w", err)
		}

		// delete original messages if needed
		if options.Delete {
			go func(messages []gotgbot.Message) {
				for _, m := range messages {
					m.Delete(bot, nil)
				}
			}(msgs)
		}

		sentMessages = append(sentMessages, msgs...)
	}
	if len(sentMessages) == 0 {
		return nil, fmt.Errorf("no messages sent")
	}

	if extractorCtx.Chat.DeleteLinks && ctx.Message != nil {
		go func(m *gotgbot.Message) {
			m.Delete(bot, nil)
		}(ctx.EffectiveMessage)
	}

	if !options.IsStored && config.Env.Caching {
		err := StoreMedia(
			extractorCtx.Context,
			extractorCtx.Extractor,
			media, sentMessages,
			formats,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to cache formats: %w", err)
		}
	}
	return sentMessages, nil
}

func chunkFormatsForUpload(formats []*models.DownloadedFormat) ([][]*models.DownloadedFormat, error) {
	if !util.IsOfficialTelegramAPI() {
		return slices.Collect(slices.Chunk(formats, 10)), nil
	}

	const multipartLimit = 50 * 1024 * 1024

	var chunks [][]*models.DownloadedFormat
	chunk := make([]*models.DownloadedFormat, 0, 10)
	var chunkSize int64

	for _, format := range formats {
		size, err := uploadSize(format)
		if err != nil {
			return nil, err
		}
		if size > multipartLimit {
			return nil, util.ErrTelegramFileTooLarge
		}

		if len(chunk) > 0 &&
			(len(chunk) == 10 || chunkSize+size > multipartLimit) {
			chunks = append(chunks, chunk)
			chunk = make([]*models.DownloadedFormat, 0, 10)
			chunkSize = 0
		}

		chunk = append(chunk, format)
		chunkSize += size
	}

	if len(chunk) > 0 {
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

func uploadProgressMessage(chunk []*models.DownloadedFormat) string {
	var totalSize int64
	for _, format := range chunk {
		totalSize += format.Format.FileSize
	}
	if totalSize <= 0 {
		return "Telegram'a yukleniyor..."
	}
	return fmt.Sprintf("Telegram'a yukleniyor... (%s)", formatBytes(totalSize))
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

func uploadSize(format *models.DownloadedFormat) (int64, error) {
	if format.Format.FileID != "" {
		return 0, nil
	}
	if format.Format.FileSize > 0 {
		return format.Format.FileSize, nil
	}
	if format.FilePath == "" {
		return 0, nil
	}
	info, err := os.Stat(format.FilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat upload file: %w", err)
	}
	format.Format.FileSize = info.Size()
	return info.Size(), nil
}

func sendSingleFormat(
	bot *gotgbot.Bot,
	chatID int64,
	format *models.DownloadedFormat,
	caption string,
	spoiler bool,
	messageOptions *gotgbot.SendMediaGroupOpts,
) (*gotgbot.Message, error) {
	media, mediaFile, err := inputFileOrID(format.Format.FileID, format.FilePath)
	if err != nil {
		return nil, err
	}
	if mediaFile != nil {
		defer mediaFile.Close()
	}

	thumbnail, thumbnailFile, err := inputFile(format.ThumbnailFilePath)
	if err != nil {
		return nil, err
	}
	if thumbnailFile != nil {
		defer thumbnailFile.Close()
	}

	_, fileType := format.Format.GetInfo()
	switch fileType {
	case models.FileTypeVideo:
		return bot.SendVideo(chatID, media, &gotgbot.SendVideoOpts{
			Thumbnail:           thumbnail,
			Width:               int64(format.Format.Width),
			Height:              int64(format.Format.Height),
			Duration:            int64(format.Format.Duration),
			Caption:             caption,
			ParseMode:           gotgbot.ParseModeHTML,
			HasSpoiler:          spoiler,
			SupportsStreaming:   true,
			DisableNotification: disableNotification(messageOptions),
			ReplyParameters:     replyParameters(messageOptions),
		})
	case models.FileTypeAudio:
		return bot.SendAudio(chatID, media, &gotgbot.SendAudioOpts{
			Thumbnail:           thumbnail,
			Duration:            int64(format.Format.Duration),
			Performer:           format.Format.Artist,
			Title:               format.Format.Title,
			Caption:             caption,
			ParseMode:           gotgbot.ParseModeHTML,
			DisableNotification: disableNotification(messageOptions),
			ReplyParameters:     replyParameters(messageOptions),
		})
	case models.FileTypePhoto:
		return bot.SendPhoto(chatID, media, &gotgbot.SendPhotoOpts{
			Caption:             caption,
			ParseMode:           gotgbot.ParseModeHTML,
			HasSpoiler:          spoiler,
			DisableNotification: disableNotification(messageOptions),
			ReplyParameters:     replyParameters(messageOptions),
		})
	case models.FileTypeDocument:
		return bot.SendDocument(chatID, media, &gotgbot.SendDocumentOpts{
			Thumbnail:           thumbnail,
			Caption:             caption,
			ParseMode:           gotgbot.ParseModeHTML,
			DisableNotification: disableNotification(messageOptions),
			ReplyParameters:     replyParameters(messageOptions),
		})
	default:
		return nil, fmt.Errorf("unknown input type: %s", fileType)
	}
}

func inputFileOrID(fileID string, filePath string) (gotgbot.InputFileOrString, *os.File, error) {
	if fileID != "" {
		return gotgbot.InputFileByID(fileID), nil, nil
	}
	if useLocalFilePathUpload() && filePath != "" {
		return gotgbot.InputFileByID(localUploadPath(filePath)), nil, nil
	}
	input, file, err := inputFile(filePath)
	return input, file, err
}

func inputFile(filePath string) (gotgbot.InputFile, *os.File, error) {
	if filePath == "" {
		return nil, nil, nil
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	return gotgbot.InputFileByReader(filepath.Base(filePath), file), file, nil
}

func disableNotification(options *gotgbot.SendMediaGroupOpts) bool {
	return options != nil && options.DisableNotification
}

func replyParameters(options *gotgbot.SendMediaGroupOpts) *gotgbot.ReplyParameters {
	if options == nil {
		return nil
	}
	return options.ReplyParameters
}

func SendInlineFormats(
	bot *gotgbot.Bot,
	ctx *ext.Context,
	extractorCtx *models.ExtractorContext,
	media *models.Media,
	formats []*models.DownloadedFormat,
	options *models.SendFormatsOptions,
) error {
	messages, err := SendFormats(
		bot, ctx, extractorCtx,
		media, formats,
		&models.SendFormatsOptions{
			Caption:  options.Caption,
			IsStored: options.IsStored,
			Delete:   true,
		},
	)
	if err != nil {
		return err
	}

	msg := messages[0]
	format := formats[0]
	fileID := util.GetMessageFileID(&msg)
	format.Format.FileID = fileID

	inputMedia, err := format.Format.GetInputMedia(
		format.FilePath, format.ThumbnailFilePath,
		options.Caption, options.IsSpoiler,
		useLocalFilePathUpload(),
	)
	if err != nil {
		return err
	}

	_, _, err = bot.EditMessageMedia(
		inputMedia,
		&gotgbot.EditMessageMediaOpts{
			InlineMessageId: ctx.ChosenInlineResult.InlineMessageId,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func useLocalFilePathUpload() bool {
	return !util.IsOfficialTelegramAPI()
}

func localUploadPath(filePath string) string {
	absolutePath, err := filepath.Abs(filePath)
	if err != nil {
		return filePath
	}
	return absolutePath
}
