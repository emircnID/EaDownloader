package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"eadownloader/internal/database"
	"eadownloader/internal/models"
	"eadownloader/internal/util"
	"eadownloader/internal/util/download"
	"eadownloader/internal/util/libav"
)

func downloadMediaFormats(
	ctx *models.ExtractorContext,
	media *models.Media,
) ([]*models.DownloadedFormat, error) {
	var wg sync.WaitGroup

	ctx.DownloadFunc = downloadFormat

	numItems := len(media.Items)
	formats := make(chan *models.DownloadedFormat, numItems)
	semaphore := make(chan struct{}, 3)

	wg.Add(numItems)
	for i := range numItems {
		go func(index int) {
			defer wg.Done()
			semaphore <- struct{}{}        // acquire
			defer func() { <-semaphore }() // release
			downloadItem(ctx, formats, media.Items[index], index)
		}(i)
	}

	// close chunks channel when all downloads complete
	go func() {
		wg.Wait()
		close(formats)
	}()

	return collectDownloadedFormats(formats, numItems)
}

func downloadItem(
	ctx *models.ExtractorContext,
	formats chan<- *models.DownloadedFormat,
	item *models.MediaItem,
	index int,
) {
	var format *models.MediaFormat

	switch len(item.Formats) {
	case 0:
		formats <- &models.DownloadedFormat{
			Index: index,
			Error: fmt.Errorf("no formats found for media item at index %d", index),
		}
		return
	case 1:
		format = item.Formats[0]
	default:
		format = item.GetDefaultFormat()
	}

	if format == nil {
		formats <- &models.DownloadedFormat{
			Index: index,
			Error: fmt.Errorf("no default format found for media item at index %d", index),
		}
		return
	}

	ctx.Debugf("selected format: %s", format.ToString())

	// validate format before download
	// to avoid downloading large files
	// or unsupported formats
	err := validateFormat(format)
	if err != nil {
		formats <- &models.DownloadedFormat{
			Index: index,
			Error: err,
		}
		return
	}

	var downloadedFormat *models.DownloadedFormat
	downloadedFormat, err = downloadMergedVideoAudioFormats(ctx, index, item, format)
	if err != nil {
		formats <- &models.DownloadedFormat{
			Index: index,
			Error: err,
		}
		return
	}
	if downloadedFormat == nil {
		ctx.Progress("Medya indiriliyor...")
		downloadedFormat, err = downloadFormat(ctx, index, format)
	}
	if err != nil {
		formats <- &models.DownloadedFormat{
			Index: index,
			Error: err,
		}
		return
	}

	// validate format again after download
	// in case metadata extraction is done
	// after download
	err = validateFormat(format)
	if err != nil {
		formats <- &models.DownloadedFormat{
			Index: index,
			Error: err,
		}
		return
	}

	if downloadedFormat.Format.AudioCodec == "" {
		// merge audio into video if needed
		mergeFormats(item, downloadedFormat)
	}

	for _, plugin := range format.Plugins {
		if plugin != nil {
			ctx.Debugf("running plugin: %s", plugin.ID)
			err := plugin.RunFunc(ctx, item, downloadedFormat)
			if err != nil {
				formats <- &models.DownloadedFormat{
					Index: index,
					Error: fmt.Errorf("plugin %s failed: %w", plugin.ID, err),
				}
				return
			}
		}
	}

	formats <- downloadedFormat
}

func downloadFormat(
	ctx *models.ExtractorContext,
	index int,
	format *models.MediaFormat,
) (*models.DownloadedFormat, error) {
	if len(format.URL) == 0 && !isCobaltDownload(format) && !isYtDLPDownload(format) {
		return nil, fmt.Errorf("no URL found for selected format")
	}

	fileName := format.GetFileName()
	var filePath string
	var thumbnailFilePath string

	// for images, download in memory and convert to jpeg
	if format.Type == database.MediaTypePhoto {
		file, err := download.DownloadFileInMemory(
			ctx, format.URL,
			format.DownloadSettings,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to download image: %w", err)
		}

		filePath = download.ToPath(fileName)
		ctx.FilesTracker.Add(filePath)

		bounds, err := util.ImgToJPEG(file, filePath, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to convert image: %w", err)
		}
		format.Width = bounds.W
		format.Height = bounds.H
		if err := setDownloadedFileSize(format, filePath); err != nil {
			return nil, err
		}
		if err := validateFormat(format); err != nil {
			return nil, err
		}

		return &models.DownloadedFormat{
			Format:   format,
			Index:    index,
			FilePath: filePath,
		}, nil
	}

	// for video and audio, download to file
	var err error
	switch {
	case isCobaltDownload(format):
		filePath, err = download.DownloadFileWithCobalt(
			ctx,
			fileName,
			format.DownloadSettings,
		)
	case isYtDLPDownload(format):
		ctx.Progress("yt-dlp ile indiriliyor...")
		filePath, err = download.DownloadFileWithYtDLP(
			ctx,
			fileName,
			format.DownloadSettings,
		)
	case len(format.Segments) > 0:
		if format.DownloadSettings != nil {
			// add decryption key to download settings if present
			format.DownloadSettings.DecryptionKey = format.DecryptionKey
		}
		filePath, err = download.DownloadFileWithSegments(
			ctx, format.InitSegment,
			format.Segments,
			fileName,
			format.DownloadSettings,
		)
	default:
		filePath, err = download.DownloadFile(
			ctx, format.URL,
			fileName, format.DownloadSettings,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	if err := setDownloadedFileSize(format, filePath); err != nil {
		return nil, err
	}
	if err := validateFormat(format); err != nil {
		return nil, err
	}

	if !skipThumbnail(format) {
		thumbnailFilePath, err = getThumbnail(ctx, format, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get thumbnail: %w", err)
		}
	}

	if format.MissingMetadata() {
		// extract video metadata if missing
		// width, height, duration
		// this is needed for Telegram video messages
		// and for validating the format
		insertVideoInfo(format, filePath)
	}

	return &models.DownloadedFormat{
		Format:            format,
		Index:             index,
		FilePath:          filePath,
		ThumbnailFilePath: thumbnailFilePath,
	}, nil
}

func downloadMergedVideoAudioFormats(
	ctx *models.ExtractorContext,
	index int,
	item *models.MediaItem,
	videoFormat *models.MediaFormat,
) (*models.DownloadedFormat, error) {
	if videoFormat.Type != database.MediaTypeVideo || videoFormat.AudioCodec != "" {
		return nil, nil
	}

	audioFormat := item.GetDefaultAudioFormat()
	if audioFormat == nil {
		return nil, nil
	}

	ctx.Progress("Video ve ses indiriliyor...")

	videoResult := make(chan *models.DownloadedFormat, 1)
	audioResult := make(chan *models.DownloadedFormat, 1)

	go func() {
		videoResult <- downloadWithResult(ctx, index, videoFormat)
	}()
	go func() {
		audioResult <- downloadWithResult(ctx, index, mergeAudioDownloadFormat(audioFormat))
	}()

	downloadedVideo := <-videoResult
	downloadedAudio := <-audioResult

	if downloadedVideo.Error != nil {
		return nil, downloadedVideo.Error
	}
	if downloadedAudio.Error != nil {
		return nil, downloadedAudio.Error
	}

	ctx.Progress("Video ve ses birlestiriliyor...")

	outputPath := strings.TrimSuffix(
		downloadedVideo.FilePath,
		filepath.Ext(downloadedVideo.FilePath),
	) + "_merged" + filepath.Ext(downloadedVideo.FilePath)
	ctx.FilesTracker.Add(outputPath)

	if err := libav.MergeVideoWithAudio(
		downloadedVideo.FilePath,
		downloadedAudio.FilePath,
		outputPath,
	); err != nil {
		return nil, fmt.Errorf("failed to merge video with audio: %w", err)
	}

	if err := os.Rename(outputPath, downloadedVideo.FilePath); err != nil {
		return nil, fmt.Errorf("failed to replace merged file: %w", err)
	}

	downloadedVideo.Format.AudioCodec = audioFormat.AudioCodec
	if err := setDownloadedFileSize(downloadedVideo.Format, downloadedVideo.FilePath); err != nil {
		return nil, err
	}
	if err := validateFormat(downloadedVideo.Format); err != nil {
		return nil, err
	}

	return downloadedVideo, nil
}

func downloadWithResult(
	ctx *models.ExtractorContext,
	index int,
	format *models.MediaFormat,
) *models.DownloadedFormat {
	downloadedFormat, err := downloadFormat(ctx, index, format)
	if err != nil {
		return &models.DownloadedFormat{
			Index: index,
			Error: err,
		}
	}
	return downloadedFormat
}

func mergeAudioDownloadFormat(format *models.MediaFormat) *models.MediaFormat {
	clone := *format
	settings := cloneDownloadSettings(format.DownloadSettings)
	settings.SkipThumbnail = true
	settings.SkipRemux = true
	clone.DownloadSettings = settings
	return &clone
}

func cloneDownloadSettings(settings *models.DownloadSettings) *models.DownloadSettings {
	if settings == nil {
		return &models.DownloadSettings{}
	}
	clone := *settings
	return &clone
}

func skipThumbnail(format *models.MediaFormat) bool {
	return format.DownloadSettings != nil && format.DownloadSettings.SkipThumbnail
}

func isCobaltDownload(format *models.MediaFormat) bool {
	return format.DownloadSettings != nil && format.DownloadSettings.CobaltURL != ""
}

func isYtDLPDownload(format *models.MediaFormat) bool {
	return format.DownloadSettings != nil &&
		format.DownloadSettings.YtDLPURL != "" &&
		format.DownloadSettings.YtDLPFormat != ""
}

func collectDownloadedFormats(
	formats chan *models.DownloadedFormat,
	numItems int,
) ([]*models.DownloadedFormat, error) {
	downloadedFormats := make([]*models.DownloadedFormat, numItems)

	var firstErr error
	formatsReceived := 0

	for df := range formats {
		formatsReceived++
		downloadedFormats[df.Index] = df
		if df.Error != nil && firstErr == nil {
			firstErr = df.Error
		}
		if formatsReceived == numItems {
			break
		}
	}

	return downloadedFormats, firstErr
}

func setDownloadedFileSize(format *models.MediaFormat, filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat downloaded file: %w", err)
	}
	format.FileSize = info.Size()
	return nil
}
