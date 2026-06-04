package youtube

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"eadownloader/internal/database"
	"eadownloader/internal/models"
	"eadownloader/internal/plugins"
	"eadownloader/internal/util"

	"github.com/bytedance/sonic"
)

const (
	format360 = "360"
	format720 = "720"
	format1080 = "1080"
	formatAudio = "audio"
	formatMP3 = "mp3"
)

var qualityTargets = []int32{360, 720, 1080}

var Extractor = &models.Extractor{
	ID:          "youtube",
	DisplayName: "YouTube",

	URLPattern: regexp.MustCompile(
		`https?://(?:(?:www|m|music)\.)?(?:(?:youtube\.com/(?:watch\?(?:[^#\s&]+&)*v=|shorts/|embed/|live/))|(?:youtu\.be/))(?P<id>[A-Za-z0-9_-]{11})`,
	),
	Host: []string{"youtube", "youtu"},

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		media, err := GetMedia(ctx)
		if err != nil {
			return nil, err
		}
		return &models.ExtractorResponse{Media: media}, nil
	},
}

func GetMedia(ctx *models.ExtractorContext) (*models.Media, error) {
	info, err := FetchInfo(ctx)
	if err != nil {
		return nil, err
	}
	return BuildMedia(ctx, info)
}

func FetchInfo(ctx *models.ExtractorContext) (*Info, error) {
	cmd := exec.CommandContext(
		ctx.Context,
		"yt-dlp",
		"--dump-single-json",
		"--no-playlist",
		"--no-warnings",
		ctx.ContentURL,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("yt-dlp failed: %s", msg)
	}

	var info Info
	if err := sonic.ConfigFastest.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse yt-dlp output: %w", err)
	}
	info.RequestedID = ctx.ContentID
	if info.ID == "" {
		info.ID = ctx.ContentID
	}
	if info.WebpageURL == "" {
		info.WebpageURL = ctx.ContentURL
	}
	return &info, nil
}

func BuildMedia(ctx *models.ExtractorContext, info *Info) (*models.Media, error) {
	media := ctx.NewMedia()
	media.ContentID = info.ID
	media.ContentURL = info.WebpageURL
	media.SetCaption(info.Title)

	item := media.NewItem()

	mergeAudioFormat := bestMergeAudioFormat(info)
	audioFormat := bestAudioFormat(info)
	for _, target := range qualityTargets {
		format := bestVideoFormat(info, target)
		if format == nil {
			continue
		}
		if !hasAudio(format) && mergeAudioFormat == nil {
			continue
		}
		mediaFormat := videoMediaFormat(info, format, target)
		item.AddFormats(mediaFormat)
	}

	if mergeAudioFormat != nil {
		item.AddFormats(mergeAudioMediaFormat(info, mergeAudioFormat))
	}
	if audioFormat != nil {
		item.AddFormats(mp3AudioMediaFormat(info, audioFormat))
	}

	if len(item.Formats) == 0 {
		return nil, fmt.Errorf("no downloadable youtube formats found")
	}
	return media, nil
}

func AvailableFormatIDs(media *models.Media) []string {
	if media == nil || len(media.Items) == 0 {
		return nil
	}
	item := media.Items[0]
	formatIDs := []string{format360, format720, format1080, formatMP3}
	available := make([]string, 0, len(formatIDs))
	for _, formatID := range formatIDs {
		if item.GetFormatByID(formatID) != nil {
			available = append(available, formatID)
		}
	}
	return available
}

func SelectMedia(media *models.Media, formatID string) (*models.Media, error) {
	if media == nil || len(media.Items) == 0 {
		return nil, fmt.Errorf("youtube media not found")
	}
	item := media.Items[0]
	selected := item.GetFormatByID(formatID)
	if selected == nil {
		return nil, fmt.Errorf("selected youtube format not found: %s", formatID)
	}

	selectedMedia := &models.Media{
		ContentID:   media.ContentID + "/" + formatID,
		ContentURL:  media.ContentURL,
		ExtractorID: media.ExtractorID,
		Caption:     media.Caption,
		NSFW:        media.NSFW,
	}
	selectedItem := selectedMedia.NewItem()
	selectedItem.AddFormats(cloneFormat(selected))

	if selected.Type == database.MediaTypeVideo && selected.AudioCodec == "" {
		if audio := item.GetFormatByID(formatAudio); audio != nil {
			selectedItem.AddFormats(cloneFormat(audio))
		}
	}

	return selectedMedia, nil
}

func bestVideoFormat(info *Info, target int32) *Format {
	candidates := make([]*Format, 0, len(info.Formats))
	for i := range info.Formats {
		format := &info.Formats[i]
		if !isDownloadable(format) || !hasVideo(format) {
			continue
		}
		if qualityHeight(format) != target {
			continue
		}
		if util.ParseVideoCodec(format.VideoCodec) != database.MediaCodecAvc {
			continue
		}
		if format.Ext != "mp4" {
			continue
		}
		candidates = append(candidates, format)
	}
	if len(candidates) == 0 {
		return nil
	}
	slices.SortFunc(candidates, func(a, b *Format) int {
		aQuality := qualityHeight(a)
		bQuality := qualityHeight(b)
		if aQuality != bQuality {
			if aQuality > bQuality {
				return -1
			}
			return 1
		}
		if a.TBR > b.TBR {
			return -1
		}
		if a.TBR < b.TBR {
			return 1
		}
		return 0
	})
	return candidates[0]
}

func bestAudioFormat(info *Info) *Format {
	candidates := make([]*Format, 0, len(info.Formats))
	for i := range info.Formats {
		format := &info.Formats[i]
		if !isDownloadable(format) || hasVideo(format) || !hasAudio(format) {
			continue
		}
		audioCodec := util.ParseAudioCodec(format.AudioCodec)
		if audioCodec != database.MediaCodecAac && audioCodec != database.MediaCodecOpus {
			continue
		}
		candidates = append(candidates, format)
	}
	if len(candidates) == 0 {
		return nil
	}
	slices.SortFunc(candidates, func(a, b *Format) int {
		aCodec := util.ParseAudioCodec(a.AudioCodec)
		bCodec := util.ParseAudioCodec(b.AudioCodec)
		if aCodec == database.MediaCodecAac && bCodec != database.MediaCodecAac {
			return -1
		}
		if aCodec != database.MediaCodecAac && bCodec == database.MediaCodecAac {
			return 1
		}
		if a.TBR > b.TBR {
			return -1
		}
		if a.TBR < b.TBR {
			return 1
		}
		return 0
	})
	return candidates[0]
}

func bestMergeAudioFormat(info *Info) *Format {
	candidates := make([]*Format, 0, len(info.Formats))
	for i := range info.Formats {
		format := &info.Formats[i]
		if !isDownloadable(format) || hasVideo(format) || !hasAudio(format) {
			continue
		}
		if util.ParseAudioCodec(format.AudioCodec) != database.MediaCodecAac {
			continue
		}
		candidates = append(candidates, format)
	}
	if len(candidates) == 0 {
		return nil
	}
	slices.SortFunc(candidates, func(a, b *Format) int {
		if a.TBR > b.TBR {
			return -1
		}
		if a.TBR < b.TBR {
			return 1
		}
		return 0
	})
	return candidates[0]
}

func videoMediaFormat(info *Info, format *Format, target int32) *models.MediaFormat {
	return &models.MediaFormat{
		FormatID:     fmt.Sprintf("%d", target),
		Type:         database.MediaTypeVideo,
		VideoCodec:   util.ParseVideoCodec(format.VideoCodec),
		AudioCodec:   audioCodec(format),
		URL:          []string{format.URL},
		ThumbnailURL: thumbnailURL(info),
		Width:        format.Width,
		Height:       format.Height,
		Duration:     int32(info.Duration),
		Bitrate:      int64(format.TBR * 1000),
		FileSize:     fileSize(format),
		DownloadSettings: &models.DownloadSettings{
			Headers: downloadHeaders(),
			Retries: 3,
		},
	}
}

func mergeAudioMediaFormat(info *Info, format *Format) *models.MediaFormat {
	return &models.MediaFormat{
		FormatID:     formatAudio,
		Type:         database.MediaTypeAudio,
		AudioCodec:   util.ParseAudioCodec(format.AudioCodec),
		URL:          []string{format.URL},
		ThumbnailURL: thumbnailURL(info),
		Duration:     int32(info.Duration),
		Title:        info.Title,
		Artist:       info.Uploader,
		Bitrate:      int64(format.TBR * 1000),
		FileSize:     fileSize(format),
		DownloadSettings: &models.DownloadSettings{
			Headers: downloadHeaders(),
			Retries: 3,
		},
	}
}

func mp3AudioMediaFormat(info *Info, format *Format) *models.MediaFormat {
	return &models.MediaFormat{
		FormatID:     formatMP3,
		Type:         database.MediaTypeAudio,
		AudioCodec:   database.MediaCodecMp3,
		URL:          []string{format.URL},
		ThumbnailURL: thumbnailURL(info),
		Duration:     int32(info.Duration),
		Title:        info.Title,
		Artist:       info.Uploader,
		Bitrate:      0,
		FileSize:     fileSize(format),
		DownloadSettings: &models.DownloadSettings{
			Headers: downloadHeaders(),
			Retries: 3,
		},
		Plugins: []*models.Plugin{plugins.ConvertAudioToMP3},
	}
}

func cloneFormat(format *models.MediaFormat) *models.MediaFormat {
	clone := *format
	clone.URL = slices.Clone(format.URL)
	clone.ThumbnailURL = slices.Clone(format.ThumbnailURL)
	clone.Plugins = slices.Clone(format.Plugins)
	return &clone
}

func audioCodec(format *Format) database.MediaCodec {
	if !hasAudio(format) {
		return ""
	}
	return util.ParseAudioCodec(format.AudioCodec)
}

func thumbnailURL(info *Info) []string {
	if info.Thumbnail == "" {
		return nil
	}
	return []string{info.Thumbnail}
}

func downloadHeaders() map[string]string {
	return map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"Referer":    "https://www.youtube.com/",
	}
}

func fileSize(format *Format) int64 {
	if format.Filesize > 0 {
		return format.Filesize
	}
	return format.FilesizeApprox
}

func qualityHeight(format *Format) int32 {
	if format.Width > 0 && format.Height > 0 && format.Height > format.Width {
		return format.Width
	}
	return format.Height
}

func isDownloadable(format *Format) bool {
	if format.URL == "" {
		return false
	}
	return strings.HasPrefix(format.Protocol, "http")
}

func hasVideo(format *Format) bool {
	return format.VideoCodec != "" && format.VideoCodec != "none"
}

func hasAudio(format *Format) bool {
	return format.AudioCodec != "" && format.AudioCodec != "none"
}
