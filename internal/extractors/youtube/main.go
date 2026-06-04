package youtube

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"eadownloader/internal/database"
	"eadownloader/internal/models"
	"eadownloader/internal/plugins"
	"eadownloader/internal/util"
)

var Extractor = &models.Extractor{
	ID:          "youtube",
	DisplayName: "YouTube",

	URLPattern: regexp.MustCompile(`https?://(?:(?:(?:www|m|music)\.)?youtube\.com/(?:(?:watch\?(?:[^#]*&)*v=)|shorts/|embed/|live/)|youtu\.be/)(?P<id>[a-zA-Z0-9_-]{11})`),
	Host: []string{
		"youtube",
		"youtu",
	},

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		media, err := GetMedia(ctx)
		if err != nil {
			return nil, err
		}
		return &models.ExtractorResponse{Media: media}, nil
	},
}

func GetMedia(ctx *models.ExtractorContext) (*models.Media, error) {
	playerResponse, err := GetPlayerResponse(ctx)
	if err != nil {
		return nil, err
	}

	media := ctx.NewMedia()
	media.SetCaption(playerResponse.VideoDetails.Title)

	item := media.NewItem()
	audioFormats := buildAudioFormats(playerResponse)
	videoFormats := buildVideoFormats(playerResponse, audioFormats)
	progressiveFormats := buildProgressiveFormats(playerResponse)

	item.AddFormats(progressiveFormats...)
	item.AddFormats(videoFormats...)
	item.AddFormats(audioFormats...)

	if len(item.Formats) == 0 {
		return nil, fmt.Errorf("no downloadable youtube formats found")
	}

	return media, nil
}

func buildProgressiveFormats(resp *PlayerResponse) []*models.MediaFormat {
	formats := make([]*models.MediaFormat, 0)
	for _, source := range resp.StreamingData.Formats {
		mediaURL := directFormatURL(source)
		if mediaURL == "" || !isVideoFormat(source) || !hasAudio(source) {
			continue
		}
		formats = append(formats, mediaFormatFromYouTubeFormat(resp, source, mediaURL, true))
	}
	return formats
}

func buildVideoFormats(
	resp *PlayerResponse,
	audioFormats []*models.MediaFormat,
) []*models.MediaFormat {
	formats := make([]*models.MediaFormat, 0)
	for _, source := range resp.StreamingData.AdaptiveFormats {
		mediaURL := directFormatURL(source)
		if mediaURL == "" || !isVideoFormat(source) {
			continue
		}
		format := mediaFormatFromYouTubeFormat(resp, source, mediaURL, false)
		if len(audioFormats) > 0 {
			audioFormat := bestAudioFormatForVideo(format.VideoCodec, audioFormats)
			format.AudioCodec = audioFormat.AudioCodec
			format.Plugins = []*models.Plugin{plugins.MergeAudio}
		}
		formats = append(formats, format)
	}
	return formats
}

func buildAudioFormats(resp *PlayerResponse) []*models.MediaFormat {
	formats := make([]*models.MediaFormat, 0)
	for _, source := range resp.StreamingData.AdaptiveFormats {
		mediaURL := directFormatURL(source)
		if mediaURL == "" || !isAudioFormat(source) {
			continue
		}
		audioCodec := util.ParseAudioCodec(source.MimeType)
		if audioCodec == "" {
			continue
		}
		formats = append(formats, &models.MediaFormat{
			Type:       database.MediaTypeAudio,
			FormatID:   fmt.Sprintf("audio_%d", source.Itag),
			URL:        []string{mediaURL},
			AudioCodec: audioCodec,
			Duration:   formatDurationSeconds(resp.VideoDetails.LengthSeconds, source),
			Bitrate:    source.Bitrate,
			FileSize:   parseInt64(source.ContentLength),
			DownloadSettings: &models.DownloadSettings{
				Headers: youtubeDownloadHeaders(),
			},
		})
	}
	return formats
}

func mediaFormatFromYouTubeFormat(
	resp *PlayerResponse,
	source Format,
	mediaURL string,
	hasBundledAudio bool,
) *models.MediaFormat {
	videoCodec := util.ParseVideoCodec(source.MimeType)
	audioCodec := database.MediaCodec("")
	if hasBundledAudio {
		audioCodec = util.ParseAudioCodec(source.MimeType)
	}

	return &models.MediaFormat{
		Type:         database.MediaTypeVideo,
		FormatID:     fmt.Sprintf("video_%d", source.Itag),
		URL:          []string{mediaURL},
		VideoCodec:   videoCodec,
		AudioCodec:   audioCodec,
		Duration:     formatDurationSeconds(resp.VideoDetails.LengthSeconds, source),
		ThumbnailURL: []string{bestThumbnailURL(resp)},
		Width:        source.Width,
		Height:       source.Height,
		Bitrate:      source.Bitrate,
		FileSize:     parseInt64(source.ContentLength),
		DownloadSettings: &models.DownloadSettings{
			Headers: youtubeDownloadHeaders(),
		},
	}
}

func bestAudioFormatForVideo(
	videoCodec database.MediaCodec,
	formats []*models.MediaFormat,
) *models.MediaFormat {
	preferredCodec := database.MediaCodecAac
	if videoCodec == database.MediaCodecVp9 || videoCodec == database.MediaCodecAv1 {
		preferredCodec = database.MediaCodecOpus
	}
	return sortAudioFormats(formats, preferredCodec)[0]
}

func sortAudioFormats(
	formats []*models.MediaFormat,
	preferredCodec database.MediaCodec,
) []*models.MediaFormat {
	candidates := slices.Clone(formats)
	slices.SortFunc(candidates, func(a, b *models.MediaFormat) int {
		if a.AudioCodec == preferredCodec && b.AudioCodec != preferredCodec {
			return -1
		}
		if a.AudioCodec != preferredCodec && b.AudioCodec == preferredCodec {
			return 1
		}
		if a.Bitrate > b.Bitrate {
			return -1
		}
		if a.Bitrate < b.Bitrate {
			return 1
		}
		return 0
	})
	return candidates
}

func isVideoFormat(format Format) bool {
	return strings.HasPrefix(format.MimeType, "video/")
}

func isAudioFormat(format Format) bool {
	return strings.HasPrefix(format.MimeType, "audio/")
}

func hasAudio(format Format) bool {
	return util.ParseAudioCodec(format.MimeType) != ""
}

func bestThumbnailURL(resp *PlayerResponse) string {
	thumbnails := resp.VideoDetails.Thumbnail.Thumbnails
	if len(thumbnails) == 0 {
		return ""
	}
	best := thumbnails[0]
	for _, thumbnail := range thumbnails[1:] {
		if thumbnail.Width*thumbnail.Height > best.Width*best.Height {
			best = thumbnail
		}
	}
	return best.URL
}

func youtubeDownloadHeaders() map[string]string {
	headers := youtubeWebHeaders()
	headers["Referer"] = "https://www.youtube.com/"
	return headers
}
