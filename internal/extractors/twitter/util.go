package twitter

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"eadownloader/internal/database"
	"eadownloader/internal/models"

	"github.com/bytedance/sonic"
)

const authToken = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"
const defaultTwitterReferer = "https://x.com/"

var resolutionRegex = regexp.MustCompile(`(\d+)x(\d+)`)

func BuildAPIHeaders(cookies []*http.Cookie) map[string]string {
	var csrfToken string
	for _, cookie := range cookies {
		if cookie.Name == "ct0" {
			csrfToken = cookie.Value
			break
		}
	}
	if csrfToken == "" {
		return nil
	}
	headers := map[string]string{
		"authorization":             "Bearer " + authToken,
		"x-twitter-auth-type":       "OAuth2Client",
		"x-twitter-client-language": "en",
		"x-twitter-active-user":     "yes",
	}

	if csrfToken != "" {
		headers["x-csrf-token"] = csrfToken
	}

	return headers
}

func BuildAPIQuery(tweetID string) string {
	variables := map[string]any{
		"tweetId":                tweetID,
		"withCommunity":          false,
		"includePromotedContent": false,
		"withVoice":              false,
	}

	features := map[string]any{
		"creator_subscriptions_tweet_preview_api_enabled":                         true,
		"tweetypie_unmention_optimization_enabled":                                true,
		"responsive_web_edit_tweet_api_enabled":                                   true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
		"view_counts_everywhere_api_enabled":                                      true,
		"longform_notetweets_consumption_enabled":                                 true,
		"responsive_web_twitter_article_tweet_consumption_enabled":                false,
		"tweet_awards_web_tipping_enabled":                                        false,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"responsive_web_graphql_exclude_directive_enabled":                        true,
		"verified_phone_label_enabled":                                            false,
		"responsive_web_media_download_video_enabled":                             false,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_enhance_cards_enabled":                                    false,
	}

	fieldToggles := map[string]any{
		"withArticleRichContentState": false,
	}

	variablesJSON, _ := sonic.ConfigFastest.Marshal(variables)
	featuresJSON, _ := sonic.ConfigFastest.Marshal(features)
	fieldTogglesJSON, _ := sonic.ConfigFastest.Marshal(fieldToggles)

	params := map[string]string{
		"variables":    string(variablesJSON),
		"features":     string(featuresJSON),
		"fieldToggles": string(fieldTogglesJSON),
	}

	query := url.Values{}
	for key, value := range params {
		query.Add(key, value)
	}
	return query.Encode()
}

func SanitizeCaption(caption string) string {
	if caption == "" {
		return ""
	}
	regex := regexp.MustCompile(`https?://t\.co/\S+`)
	return strings.TrimSpace(regex.ReplaceAllString(caption, ""))
}

func ExtractVideoFormats(media *MediaEntity, contentURL string) ([]*models.MediaFormat, error) {
	var formats []*models.MediaFormat

	if media.VideoInfo == nil {
		return formats, nil
	}

	duration := int32(media.VideoInfo.DurationMillis / 1000)

	for _, variant := range media.VideoInfo.Variants {
		if variant.ContentType == "video/mp4" {
			width, height := ResolutionFromURL(variant.URL)

			formats = append(formats, &models.MediaFormat{
				Type:             database.MediaTypeVideo,
				FormatID:         fmt.Sprintf("mp4_%d", variant.Bitrate),
				URL:              []string{variant.URL},
				VideoCodec:       database.MediaCodecAvc,
				AudioCodec:       database.MediaCodecAac,
				Duration:         duration,
				ThumbnailURL:     []string{media.MediaURLHTTPS},
				Width:            width,
				Height:           height,
				Bitrate:          variant.Bitrate,
				DownloadSettings: twitterDownloadSettings(contentURL),
			})
		}
	}

	return formats, nil
}

func twitterDownloadSettings(contentURL string) *models.DownloadSettings {
	referer := canonicalTwitterReferer(contentURL)

	return &models.DownloadSettings{
		Headers: map[string]string{
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36",
			"Accept":          "*/*",
			"Accept-Language": "en-US,en;q=0.9",
			"Origin":          "https://x.com",
			"Referer":         referer,
		},
		Retries:             3,
		NumConnections:      1,
		Impersonate:         true,
		SkipRemux:           true,
		SkipThumbnail:       true,
		YtDLPURL:            contentURL,
		YtDLPFormat:         "bv*+ba/b",
		YtDLPFallbackURL:    contentURL,
		YtDLPFallbackFormat: "bv*+ba/b",
	}
}

func canonicalTwitterReferer(contentURL string) string {
	if contentURL == "" {
		return defaultTwitterReferer
	}

	parsed, err := url.Parse(contentURL)
	if err != nil {
		return defaultTwitterReferer
	}

	switch parsed.Host {
	case "twitter.com", "www.twitter.com":
		parsed.Host = strings.Replace(parsed.Host, "twitter.com", "x.com", 1)
	case "x.com", "www.x.com":
	default:
		return defaultTwitterReferer
	}

	parsed.Scheme = "https"
	return parsed.String()
}

func ResolutionFromURL(url string) (int32, int32) {
	matches := resolutionRegex.FindStringSubmatch(url)
	if len(matches) >= 3 {
		width, _ := strconv.ParseInt(matches[1], 10, 32)
		height, _ := strconv.ParseInt(matches[2], 10, 32)
		return int32(width), int32(height)
	}
	return 0, 0
}
