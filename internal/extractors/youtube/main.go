package youtube

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"eadownloader/internal/database"
	"eadownloader/internal/models"
	"eadownloader/internal/util"

	"github.com/bytedance/sonic"
)

const (
	youtubeFallbackTitle = "YouTube"

	formatBest = "best"
	format360  = "360"
	format720  = "720"
	format1080 = "1080"
	formatMP3  = "mp3"
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
	return BuildFastMedia(ctx), nil
}

func BuildFastMedia(ctx *models.ExtractorContext) *models.Media {
	media := ctx.NewMedia()
	media.ContentID = ctx.ContentID
	media.ContentURL = ctx.ContentURL
	media.SetCaption(fetchOEmbedTitle(ctx))

	item := media.NewItem()
	for _, target := range qualityTargets {
		item.AddFormats(fastVideoMediaFormat(ctx.ContentURL, fmt.Sprintf("%d", target), target))
	}
	item.AddFormats(fastAudioMediaFormat(ctx.ContentURL))

	if IsShortsURL(ctx.ContentURL) {
		item.AddFormats(fastVideoMediaFormat(ctx.ContentURL, formatBest, 720))
	}

	return media
}

type oEmbedResponse struct {
	Title string `json:"title"`
}

func fetchOEmbedTitle(ctx *models.ExtractorContext) string {
	apiURL := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(ctx.ContentURL)
	resp, err := ctx.Fetch(
		http.MethodGet,
		apiURL,
		nil,
	)
	if err != nil {
		ctx.Warnf("failed to fetch youtube oembed title: %v", err)
		return youtubeFallbackTitle
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ctx.Warnf("failed to fetch youtube oembed title: %s", resp.Status)
		return youtubeFallbackTitle
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.Warnf("failed to read youtube oembed title: %v", err)
		return youtubeFallbackTitle
	}

	var data oEmbedResponse
	if err := sonic.ConfigFastest.Unmarshal(body, &data); err != nil {
		ctx.Warnf("failed to parse youtube oembed title: %v", err)
		return youtubeFallbackTitle
	}

	title := strings.TrimSpace(data.Title)
	if title == "" {
		return youtubeFallbackTitle
	}
	return title
}

func FetchInfo(ctx *models.ExtractorContext) (*Info, error) {
	var lastErr error
	for _, args := range ytDLPInfoArgs(ctx.ContentURL) {
		output, err := runYTDLP(ctx, args)
		if err != nil {
			lastErr = err
			if !isBotCheckError(err) {
				break
			}
			ctx.Warnf("youtube metadata fallback after yt-dlp error: %v", err)
			continue
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

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("yt-dlp failed")
}

func BuildMedia(ctx *models.ExtractorContext, info *Info) (*models.Media, error) {
	media := ctx.NewMedia()
	media.ContentID = info.ID
	media.ContentURL = info.WebpageURL
	media.SetCaption(info.Title)

	item := media.NewItem()

	audioFormat := bestAudioFormat(info)
	for _, target := range qualityTargets {
		format := bestVideoFormat(info, target)
		if format == nil {
			continue
		}
		mediaFormat := videoMediaFormat(info, format, target)
		item.AddFormats(mediaFormat)
	}

	if audioFormat != nil {
		item.AddFormats(mp3AudioMediaFormat(info, audioFormat))
	}

	if IsShortsURL(ctx.ContentURL) {
		format := bestAvailableVideoFormat(info)
		if format != nil {
			item.AddFormats(videoMediaFormatWithID(info, format, formatBest))
		}
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

func IsShortsURL(contentURL string) bool {
	return strings.Contains(strings.ToLower(contentURL), "youtube.com/shorts/")
}

func SelectableFormatIDs() []string {
	return []string{format360, format720, format1080, formatMP3}
}

func SelectMedia(media *models.Media, formatID string) (*models.Media, error) {
	if media == nil || len(media.Items) == 0 {
		return nil, fmt.Errorf("youtube media not found")
	}
	item := media.Items[0]
	selected := item.GetFormatByID(formatID)
	if selected == nil {
		if !isVideoFormatID(formatID) {
			return nil, fmt.Errorf("selected youtube format not found: %s", formatID)
		}

		selected = fallbackVideoFormat(item, formatID)
		if selected == nil {
			return nil, fmt.Errorf("selected youtube format not found: %s", formatID)
		}
	}

	return selectMediaFormat(media, selected)
}

func SelectBestVideoMedia(media *models.Media) (*models.Media, error) {
	if media == nil || len(media.Items) == 0 {
		return nil, fmt.Errorf("youtube media not found")
	}

	selected := media.Items[0].GetFormatByID(formatBest)
	if selected == nil {
		selected = media.Items[0].GetDefaultVideoFormat()
	}
	if selected == nil {
		return nil, fmt.Errorf("no downloadable youtube video format found")
	}

	return selectMediaFormat(media, selected)
}

func selectMediaFormat(media *models.Media, selected *models.MediaFormat) (*models.Media, error) {
	selectedMedia := &models.Media{
		ContentID:   media.ContentID + "/" + selected.FormatID,
		ContentURL:  media.ContentURL,
		ExtractorID: media.ExtractorID,
		Caption:     media.Caption,
		NSFW:        media.NSFW,
	}
	selectedItem := selectedMedia.NewItem()
	selectedItem.AddFormats(cloneFormat(selected))

	return selectedMedia, nil
}

func fallbackVideoFormat(item *models.MediaItem, formatID string) *models.MediaFormat {
	target := formatTarget(formatID)
	if target == 0 {
		return item.GetDefaultVideoFormat()
	}

	candidates := item.FilterFormats(func(format *models.MediaFormat) bool {
		return format.Type == database.MediaTypeVideo &&
			format.VideoCodec != "" &&
			formatTarget(format.FormatID) > 0 &&
			formatTarget(format.FormatID) <= target
	})
	if len(candidates) == 0 {
		return item.GetDefaultVideoFormat()
	}

	slices.SortFunc(candidates, func(a, b *models.MediaFormat) int {
		aTarget := formatTarget(a.FormatID)
		bTarget := formatTarget(b.FormatID)
		if aTarget != bTarget {
			if aTarget > bTarget {
				return -1
			}
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
	return candidates[0]
}

func isVideoFormatID(formatID string) bool {
	return formatID == format360 || formatID == format720 || formatID == format1080
}

func formatTarget(formatID string) int32 {
	switch formatID {
	case format360:
		return 360
	case format720:
		return 720
	case format1080:
		return 1080
	default:
		return 0
	}
}

func bestVideoFormat(info *Info, target int32) *Format {
	candidates := make([]*Format, 0, len(info.Formats))
	for i := range info.Formats {
		format := &info.Formats[i]
		if !hasVideo(format) {
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
		return compareSmallerVideo(a, b)
	})
	return candidates[0]
}

func bestAvailableVideoFormat(info *Info) *Format {
	candidates := make([]*Format, 0, len(info.Formats))
	for i := range info.Formats {
		format := &info.Formats[i]
		if !hasVideo(format) {
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
		return compareSmallerVideo(a, b)
	})
	return candidates[0]
}

func bestAudioFormat(info *Info) *Format {
	candidates := make([]*Format, 0, len(info.Formats))
	for i := range info.Formats {
		format := &info.Formats[i]
		if hasVideo(format) || !hasAudio(format) {
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

func videoMediaFormat(info *Info, format *Format, target int32) *models.MediaFormat {
	return videoMediaFormatWithID(info, format, fmt.Sprintf("%d", target))
}

func videoMediaFormatWithID(info *Info, format *Format, formatID string) *models.MediaFormat {
	return &models.MediaFormat{
		FormatID:         formatID,
		Type:             database.MediaTypeVideo,
		VideoCodec:       util.ParseVideoCodec(format.VideoCodec),
		AudioCodec:       database.MediaCodecAac,
		ThumbnailURL:     thumbnailURL(info),
		Width:            format.Width,
		Height:           format.Height,
		Duration:         int32(info.Duration),
		Bitrate:          int64(format.TBR * 1000),
		FileSize:         fileSize(format),
		DownloadSettings: youtubeVideoDownloadSettings(info.WebpageURL, formatID),
	}
}

func fastVideoMediaFormat(contentURL string, formatID string, target int32) *models.MediaFormat {
	return &models.MediaFormat{
		FormatID:         formatID,
		Type:             database.MediaTypeVideo,
		VideoCodec:       database.MediaCodecAvc,
		AudioCodec:       database.MediaCodecAac,
		ThumbnailURL:     nil,
		Width:            videoWidth(target),
		Height:           target,
		DownloadSettings: youtubeVideoDownloadSettings(contentURL, formatID),
	}
}

func fastAudioMediaFormat(contentURL string) *models.MediaFormat {
	return &models.MediaFormat{
		FormatID:         formatMP3,
		Type:             database.MediaTypeAudio,
		AudioCodec:       database.MediaCodecMp3,
		DownloadSettings: youtubeAudioDownloadSettings(contentURL),
	}
}

func mp3AudioMediaFormat(info *Info, format *Format) *models.MediaFormat {
	return &models.MediaFormat{
		FormatID:         formatMP3,
		Type:             database.MediaTypeAudio,
		AudioCodec:       database.MediaCodecMp3,
		ThumbnailURL:     thumbnailURL(info),
		Duration:         int32(info.Duration),
		Title:            info.Title,
		Artist:           info.Uploader,
		Bitrate:          0,
		FileSize:         fileSize(format),
		DownloadSettings: youtubeAudioDownloadSettings(info.WebpageURL),
	}
}

func cloneFormat(format *models.MediaFormat) *models.MediaFormat {
	clone := *format
	clone.URL = slices.Clone(format.URL)
	clone.ThumbnailURL = slices.Clone(format.ThumbnailURL)
	clone.Plugins = slices.Clone(format.Plugins)
	return &clone
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

func youtubeVideoDownloadSettings(contentURL string, formatID string) *models.DownloadSettings {
	target := formatTarget(formatID)
	if target == 0 {
		target = 720
	}
	settings := youtubeDownloadSettings(contentURL, youtubeVideoSelector(target, true))
	if target > 720 {
		settings.YtDLPSort = youtubeVideoSort(target)
	}
	return settings
}

func youtubeAudioDownloadSettings(contentURL string) *models.DownloadSettings {
	settings := youtubeDownloadSettings(contentURL, "bestaudio/best")
	settings.YtDLPAudio = true
	return settings
}

func youtubeDownloadSettings(contentURL string, formatSelector string) *models.DownloadSettings {
	return &models.DownloadSettings{
		Headers:        downloadHeaders(),
		NumConnections: 16,
		Retries:        3,
		SkipRemux:      true,
		SkipThumbnail:  true,
		YtDLPURL:       contentURL,
		YtDLPFormat:    formatSelector,
		YtDLPCookieJar: youtubeCookiePath(),
		YtDLPArgs:      "youtube:player_client=tv,web_embedded,android;formats=missing_pot",
	}
}

func youtubeVideoSelector(target int32, preferProgressive bool) string {
	if preferProgressive {
		switch target {
		case 360:
			return "18/best[height<=360][ext=mp4][vcodec^=avc1][acodec!=none]/best[height<=360][ext=mp4][acodec!=none]"
		case 720:
			return "22/18/best[height<=720][ext=mp4][vcodec^=avc1][acodec!=none]/best[height<=720][ext=mp4][acodec!=none]"
		}
	}
	return fmt.Sprintf(
		"bestvideo[height=%d][ext=mp4][vcodec^=avc1]+bestaudio[ext=m4a]/"+
			"bestvideo[height<=%d][ext=mp4][vcodec^=avc1]+bestaudio[ext=m4a]/"+
			"best[height<=%d][ext=mp4]/best[height<=%d]",
		target,
		target,
		target,
		target,
	)
}

func youtubeVideoSort(target int32) string {
	return fmt.Sprintf("res:%d,+size,+br,+fps,+codec:h264:m4a", target)
}

func fileSize(format *Format) int64 {
	if format.Filesize > 0 {
		return format.Filesize
	}
	return format.FilesizeApprox
}

func compareSmallerVideo(left, right *Format) int {
	leftSize := fileSize(left)
	rightSize := fileSize(right)
	if leftSize > 0 && rightSize > 0 && leftSize != rightSize {
		if leftSize < rightSize {
			return -1
		}
		return 1
	}
	if leftSize > 0 && rightSize == 0 {
		return -1
	}
	if leftSize == 0 && rightSize > 0 {
		return 1
	}
	if left.TBR < right.TBR {
		return -1
	}
	if left.TBR > right.TBR {
		return 1
	}
	return 0
}

func videoWidth(height int32) int32 {
	if height <= 0 {
		return 0
	}
	return height * 16 / 9
}

func qualityHeight(format *Format) int32 {
	if format.Width > 0 && format.Height > 0 && format.Height > format.Width {
		return format.Width
	}
	return format.Height
}

func hasVideo(format *Format) bool {
	return format.VideoCodec != "" && format.VideoCodec != "none"
}

func hasAudio(format *Format) bool {
	return format.AudioCodec != "" && format.AudioCodec != "none"
}

func ytDLPInfoArgs(contentURL string) [][]string {
	base := []string{
		"--dump-single-json",
		"--no-playlist",
		"--no-warnings",
		"--force-ipv4",
		"--socket-timeout", "15",
	}
	if cookiePath := youtubeCookiePath(); cookiePath != "" {
		base = append(base, "--cookies", cookiePath)
	}

	return [][]string{
		append(slices.Clone(base), contentURL),
		append(
			slices.Clone(base),
			"--extractor-args",
			"youtube:player_client=tv,web_embedded,android;formats=missing_pot",
			contentURL,
		),
		append(
			slices.Clone(base),
			"--extractor-args",
			"youtube:player_client=default,-web_safari;formats=missing_pot",
			contentURL,
		),
	}
}

func runYTDLP(ctx *models.ExtractorContext, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx.Context, "yt-dlp", args...)

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
	return output, nil
}

func isBotCheckError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "sign in to confirm") ||
		strings.Contains(msg, "not a bot")
}

func youtubeCookiePath() string {
	cookiePath := filepath.Join("private", "cookies", "youtube.txt")
	if _, err := os.Stat(cookiePath); err != nil {
		return ""
	}
	return cookiePath
}
