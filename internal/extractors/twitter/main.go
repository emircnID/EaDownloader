package twitter

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"eadownloader/internal/database"
	"eadownloader/internal/logger"
	"eadownloader/internal/models"
	"eadownloader/internal/networking"
	"eadownloader/internal/util"

	"github.com/bytedance/sonic"
)

const (
	apiHostname = "x.com"
	apiBase     = "https://" + apiHostname + "/i/api/graphql/"
	apiEndpoint = apiBase + "2ICDjqPd81tulZcYrtpTuQ/TweetResultByRestId"
	fxAPIBase   = "https://api.fxtwitter.com/2/status/"
)

var ShortExtractor = &models.Extractor{
	ID:          "twitter",
	DisplayName: "Twitter (Short)",

	URLPattern: regexp.MustCompile(`https?://t\.co/(?P<id>\w+)`),
	Host:       []string{"t"},

	Redirect: true,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		resp, err := ctx.Fetch(
			http.MethodGet,
			ctx.ContentURL,
			nil,
		)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		matchedURL := Extractor.URLPattern.FindSubmatch(body)
		if matchedURL == nil {
			// not a twitter url, most likely a
			// t.co link to something else
			return nil, nil
		}
		return &models.ExtractorResponse{
			URL: string(matchedURL[0]),
		}, nil
	},
}

var Extractor = &models.Extractor{
	ID:          "twitter",
	DisplayName: "Twitter (X)",

	URLPattern: regexp.MustCompile(`https?:\/\/(?:fx|vx|fixup)?(twitter|x)\.com\/([^\/]+)\/status\/(?P<id>\d+)`),
	Host: []string{
		"x",
		"twitter",
		"fxtwitter",
		"vxtwitter",
		"fixuptwitter",
		"fixupx",
	},

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		media, err := MediaFromAPI(ctx)
		if err != nil {
			return nil, err
		}
		return &models.ExtractorResponse{
			Media: media,
		}, nil
	},
}

func MediaFromAPI(ctx *models.ExtractorContext) (*models.Media, error) {
	tweetData, err := GetTweetAPI(ctx)
	if err != nil {
		ctx.Warnf("official twitter api failed, trying public fallback: %v", err)
		return MediaFromFXAPI(ctx)
	}

	media := ctx.NewMedia()
	caption := SanitizeCaption(tweetData.FullText)
	media.SetCaption(caption)

	var mediaEntities []*MediaEntity
	switch {
	case tweetData.Entities != nil && len(tweetData.Entities.Media) > 0:
		mediaEntities = tweetData.Entities.Media
	case tweetData.ExtendedEntities != nil && len(tweetData.ExtendedEntities.Media) > 0:
		mediaEntities = tweetData.ExtendedEntities.Media
	default:
		return nil, nil
	}

	for _, mediaEntity := range mediaEntities {
		item := media.NewItem()

		switch mediaEntity.Type {
		case "video", "animated_gif":
			formats, err := ExtractVideoFormats(mediaEntity, ctx.ContentURL, ctx.HTTPClient.Cookies)
			if err != nil {
				return nil, err
			}
			item.AddFormats(formats...)
		case "photo":
			item.AddFormats(&models.MediaFormat{
				Type:             database.MediaTypePhoto,
				FormatID:         "photo",
				URL:              []string{mediaEntity.MediaURLHTTPS},
				DownloadSettings: twitterDownloadSettings(ctx.ContentURL, ctx.HTTPClient.Cookies),
			})
		}
	}

	if len(media.Items) == 0 {
		// tweet has no media
		return nil, nil
	}

	return media, nil
}

func GetTweetAPI(ctx *models.ExtractorContext) (*Tweet, error) {
	tweetID := ctx.ContentID
	if ctx.HTTPClient.Cookies == nil {
		return nil, fmt.Errorf("auth cookies are required")
	}
	headers := BuildAPIHeaders(ctx.HTTPClient.Cookies)
	if headers == nil {
		return nil, fmt.Errorf("invalid auth cookies")
	}
	query := BuildAPIQuery(tweetID)

	reqURL := apiEndpoint + "?" + query
	resp, err := ctx.Fetch(
		http.MethodGet,
		reqURL, &networking.RequestParams{
			Headers: headers,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	logger.WriteFile("twitter_api_response", resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid response code: %s", resp.Status)
	}

	var apiResponse APIResponse
	err = sonic.ConfigFastest.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := apiResponse.Data.TweetResult.Result
	if result == nil {
		return nil, util.ErrUnavailable
	}

	if result.TypeName == "TweetUnavailable" {
		return nil, util.ErrUnavailable
	}

	var tweet *Tweet
	switch {
	case result.Tweet != nil:
		tweet = result.Tweet.Legacy
	case result.Legacy != nil:
		tweet = result.Legacy
	default:
		return nil, fmt.Errorf("tweet data not found")
	}

	return tweet, nil
}

func MediaFromFXAPI(ctx *models.ExtractorContext) (*models.Media, error) {
	status, err := GetTweetFXAPI(ctx)
	if err != nil {
		return nil, err
	}
	if status.Media == nil {
		return nil, nil
	}

	media := ctx.NewMedia()
	media.SetCaption(SanitizeCaption(status.Text))

	for _, photo := range status.Media.Photos {
		if photo.URL == "" {
			continue
		}
		item := media.NewItem()
		item.AddFormats(&models.MediaFormat{
			Type:             database.MediaTypePhoto,
			FormatID:         nonEmpty(photo.ID, "photo"),
			URL:              []string{photo.URL},
			Width:            photo.Width,
			Height:           photo.Height,
			DownloadSettings: twitterDownloadSettings(ctx.ContentURL, ctx.HTTPClient.Cookies),
		})
	}

	for _, video := range status.Media.Videos {
		formats := formatsFromFXVideo(video, ctx.ContentURL, ctx.HTTPClient.Cookies)
		if len(formats) == 0 && video.URL != "" {
			formats = append(formats, &models.MediaFormat{
				Type:             database.MediaTypeVideo,
				FormatID:         nonEmpty(video.ID, "video"),
				URL:              []string{video.URL},
				VideoCodec:       database.MediaCodecAvc,
				AudioCodec:       database.MediaCodecAac,
				Duration:         int32(video.Duration),
				ThumbnailURL:     []string{video.ThumbnailURL},
				Width:            video.Width,
				Height:           video.Height,
				FileSize:         video.Filesize,
				DownloadSettings: twitterDownloadSettings(ctx.ContentURL, ctx.HTTPClient.Cookies),
			})
		}
		if len(formats) == 0 {
			continue
		}
		item := media.NewItem()
		item.AddFormats(formats...)
	}

	if len(media.Items) == 0 {
		return nil, nil
	}

	return media, nil
}

func GetTweetFXAPI(ctx *models.ExtractorContext) (*FXStatus, error) {
	resp, err := ctx.Fetch(
		http.MethodGet,
		fxAPIBase+ctx.ContentID,
		&networking.RequestParams{
			Headers: map[string]string{
				"Accept": "application/json",
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch fxtwitter api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid fxtwitter response code: %s", resp.Status)
	}

	var apiResponse FXAPIResponse
	if err := sonic.ConfigFastest.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse fxtwitter response: %w", err)
	}
	if apiResponse.Code == http.StatusUnauthorized {
		return nil, util.ErrAuthenticationNeeded
	}
	if apiResponse.Code == http.StatusNotFound {
		return nil, util.ErrUnavailable
	}
	if apiResponse.Code != http.StatusOK || apiResponse.Status == nil {
		if apiResponse.Message != "" {
			return nil, fmt.Errorf("fxtwitter error: %s", apiResponse.Message)
		}
		return nil, fmt.Errorf("fxtwitter returned code %d", apiResponse.Code)
	}

	return apiResponse.Status, nil
}

func formatsFromFXVideo(video FXVideo, contentURL string, cookies []*http.Cookie) []*models.MediaFormat {
	formats := make([]*models.MediaFormat, 0, len(video.Formats))
	duration := int32(video.Duration)

	for _, source := range video.Formats {
		if source.URL == "" {
			continue
		}
		videoCodec := fxVideoCodec(source.Codec)
		if videoCodec == "" && source.Container != "mp4" {
			videoCodec = database.MediaCodecVp9
		}
		if videoCodec == "" {
			videoCodec = database.MediaCodecAvc
		}
		formatIDParts := []string{source.Container, source.Codec}
		if source.Height > 0 {
			formatIDParts = append(formatIDParts, fmt.Sprintf("%dp", source.Height))
		}
		formatID := strings.Trim(strings.Join(formatIDParts, "_"), "_")
		if formatID == "" {
			formatID = nonEmpty(video.ID, "video")
		}

		formats = append(formats, &models.MediaFormat{
			Type:             database.MediaTypeVideo,
			FormatID:         formatID,
			URL:              []string{source.URL},
			VideoCodec:       videoCodec,
			AudioCodec:       database.MediaCodecAac,
			Duration:         duration,
			ThumbnailURL:     []string{video.ThumbnailURL},
			Width:            firstInt32(source.Width, video.Width),
			Height:           firstInt32(source.Height, video.Height),
			Bitrate:          source.Bitrate,
			FileSize:         firstInt64(source.Size, video.Filesize),
			DownloadSettings: twitterDownloadSettings(contentURL, cookies),
		})
	}
	return formats
}

func fxVideoCodec(codec string) database.MediaCodec {
	switch strings.ToLower(codec) {
	case "h264", "avc":
		return database.MediaCodecAvc
	case "hevc", "h265":
		return database.MediaCodecHevc
	case "vp9":
		return database.MediaCodecVp9
	case "av1":
		return database.MediaCodecAv1
	default:
		return ""
	}
}

func nonEmpty(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func firstInt32(values ...int32) int32 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
