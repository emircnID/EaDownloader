package facebook

import (
	"fmt"
	"regexp"

	"eadownloader/internal/database"
	"eadownloader/internal/models"
	"eadownloader/internal/networking"
)

var facebookHost = []string{"facebook"}

var WatchShortExtractor = &models.Extractor{
	ID:          "facebook",
	DisplayName: "Facebook (Short)",

	URLPattern: regexp.MustCompile(`https?://fb\.watch/(?P<id>[a-zA-Z0-9_-]+)`),
	Host:       []string{"fb"},

	Redirect: true,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		finalURL, err := ctx.FetchLocation(
			ctx.ContentURL,
			&networking.RequestParams{Headers: webHeaders},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to follow short redirect: %w", err)
		}
		return &models.ExtractorResponse{URL: finalURL}, nil
	},
}

var ShareExtractor = &models.Extractor{
	ID:          "facebook",
	DisplayName: "Facebook (Share)",

	URLPattern: regexp.MustCompile(`https?://(?:(?:www|m)\.)?facebook\.com/share/(?:r|v|p)/(?P<id>[a-zA-Z0-9_-]+)`),
	Host:       facebookHost,

	Redirect: true,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		finalURL, err := ctx.FetchLocation(
			ctx.ContentURL,
			&networking.RequestParams{Headers: webHeaders},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to follow share redirect: %w", err)
		}
		return &models.ExtractorResponse{URL: finalURL}, nil
	},
}

var Extractor = &models.Extractor{
	ID:          "facebook",
	DisplayName: "Facebook",

	URLPattern: regexp.MustCompile(
		`https?://(?:(?:www|m|mbasic)\.)?facebook\.com/` +
			`(?:watch/?\?(?:[^&]*&)*v=|(?:reel|videos?|posts?|photo)/|permalink\.php\?(?:[^#\s&]+&)*story_fbid=|photo\.php\?(?:[^#\s&]+&)*fbid=|[^/]+/(?:videos|posts|reels?)/)` +
			`(?P<id>[a-zA-Z0-9]+)`,
	),
	Host: facebookHost,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		media, err := GetMedia(ctx)
		if err != nil {
			return nil, err
		}
		return &models.ExtractorResponse{
			Media: media,
		}, nil
	},
}

func GetMedia(ctx *models.ExtractorContext) (*models.Media, error) {
	videoData, err := GetVideoData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get media data: %w", err)
	}
	return buildMedia(ctx, videoData)
}

func buildMedia(ctx *models.ExtractorContext, data *VideoData) (*models.Media, error) {
	media := ctx.NewMedia()
	if data.Title != "" {
		media.SetCaption(data.Title)
	}

	var formats []*models.MediaFormat

	if data.HDURL != "" {
		formats = append(formats, &models.MediaFormat{
			FormatID:   "hd",
			Type:       database.MediaTypeVideo,
			VideoCodec: database.MediaCodecAvc,
			AudioCodec: database.MediaCodecAac,
			URL:        []string{data.HDURL},
			Width:      data.Width,
			Height:     data.Height,
		})
	}
	if data.SDURL != "" {
		formats = append(formats, &models.MediaFormat{
			FormatID:   "sd",
			Type:       database.MediaTypeVideo,
			VideoCodec: database.MediaCodecAvc,
			AudioCodec: database.MediaCodecAac,
			URL:        []string{data.SDURL},
		})
	}

	if len(formats) > 0 {
		item := media.NewItem()
		item.AddFormats(formats...)
		return media, nil
	}

	for _, imageURL := range data.ImageURLs {
		item := media.NewItem()
		item.AddFormats(&models.MediaFormat{
			FormatID: "image",
			Type:     database.MediaTypePhoto,
			URL:      []string{imageURL},
		})
	}

	if len(media.Items) == 0 {
		return nil, fmt.Errorf("no media found")
	}
	return media, nil
}
