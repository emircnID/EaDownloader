package instagram

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"eadownloader/internal/database"
	"eadownloader/internal/logger"
	"eadownloader/internal/models"
	"eadownloader/internal/networking"
	"eadownloader/internal/util"

	"github.com/bytedance/sonic"
	"github.com/titanous/json5"
)

const (
	graphQLEndpoint = "https://www.instagram.com/graphql/query/"
	polarisAction   = "PolarisPostActionLoadPostQueryQuery"

	igramHostname = "api-wh.igram.world"
	igramAPIBase  = "api.igram.world"
	igramHMACKey  = "75f2d70d3724f98e4a7d1ffd0ba9cfd907f3ae2632ee159980e2c521bff62358"
	igramStaticTS = 1771418815381 // parseInt("mls10xp1", 36)
)

var (
	embedPattern = regexp.MustCompile(
		`new ServerJS\(\)\);s\.handle\(({.*})\);requireLazy`)

	webHeaders = map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":           "en-GB,en;q=0.9",
		"Cache-Control":             "max-age=0",
		"Dnt":                       "1",
		"Priority":                  "u=0, i",
		"Sec-Ch-Ua":                 `Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99`,
		"Sec-Ch-Ua-Mobile":          "?0",
		"Sec-Ch-Ua-Platform":        "macOS",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
	}

	igramHeaders = map[string]string{
		"Referer": "https://igram.world/",
	}
)

func ParseGQLMedia(ctx *models.ExtractorContext, data *Media) (*models.Media, error) {
	if data == nil {
		return nil, fmt.Errorf("media data is nil")
	}

	media := ctx.NewMedia()
	media.SetCaption(mediaCaption(data))

	switch data.Typename {
	case "GraphVideo", "XDTGraphVideo":
		addGQLVideoFormat(media, data)
	case "GraphImage", "XDTGraphImage":
		addGQLImageFormat(media, data.DisplayURL)
	case "GraphSidecar", "XDTGraphSidecar":
		if data.EdgeSidecarToChildren != nil && len(data.EdgeSidecarToChildren.Edges) > 0 {
			edges := data.EdgeSidecarToChildren.Edges

			for i := range edges {
				if edges[i] == nil {
					continue
				}
				node := edges[i].Node
				if node == nil {
					continue
				}

				switch node.Typename {
				case "GraphVideo", "XDTGraphVideo":
					addGQLVideoFormat(media, node)
				case "GraphImage", "XDTGraphImage":
					addGQLImageFormat(media, node.DisplayURL)
				}
			}
		}
	}

	if len(media.Items) == 0 {
		return nil, fmt.Errorf("no downloadable media found")
	}

	return media, nil
}

func addGQLVideoFormat(media *models.Media, data *Media) {
	videoURL := strings.TrimSpace(data.VideoURL)
	if !isHTTPURL(videoURL) {
		return
	}

	item := media.NewItem()
	format := &models.MediaFormat{
		FormatID:         "video",
		Type:             database.MediaTypeVideo,
		VideoCodec:       database.MediaCodecAvc,
		AudioCodec:       database.MediaCodecAac,
		URL:              []string{videoURL},
		DownloadSettings: instagramDownloadSettings(),
	}
	if data.Dimensions != nil {
		format.Width = data.Dimensions.Width
		format.Height = data.Dimensions.Height
	}
	if thumbnailURL := strings.TrimSpace(data.DisplayURL); isHTTPURL(thumbnailURL) {
		format.ThumbnailURL = []string{thumbnailURL}
	}
	item.AddFormats(format)
}

func addGQLImageFormat(media *models.Media, imageURL string) {
	imageURL = strings.TrimSpace(imageURL)
	if !isHTTPURL(imageURL) {
		return
	}
	item := media.NewItem()
	item.AddFormats(&models.MediaFormat{
		FormatID:         "image",
		Type:             database.MediaTypePhoto,
		URL:              []string{imageURL},
		DownloadSettings: instagramDownloadSettings(),
	})
}

func mediaCaption(data *Media) string {
	if data == nil {
		return ""
	}
	if data.EdgeMediaToCaption != nil {
		for _, edge := range data.EdgeMediaToCaption.Edges {
			if edge == nil || edge.Node == nil {
				continue
			}
			if text := strings.TrimSpace(edge.Node.Text); text != "" && !isFormatName(text) {
				return text
			}
		}
	}
	title := strings.TrimSpace(data.Title)
	if isFormatName(title) {
		return ""
	}
	return title
}

func contextCaption(ctxJSON *ContextJSON) string {
	if ctxJSON == nil || ctxJSON.Context == nil {
		return ""
	}
	for _, candidate := range []string{
		ctxJSON.Context.Caption,
		ctxJSON.Context.CaptionTitleLinkified,
		ctxJSON.Context.Title,
		ctxJSON.Context.AltText,
	} {
		if text := strings.TrimSpace(candidate); text != "" && !isFormatName(text) {
			return text
		}
	}
	return ""
}

func igramCaption(details *IGramResponse) string {
	if details == nil {
		return ""
	}
	for _, item := range details.Items {
		if item == nil {
			continue
		}
		for _, candidate := range []string{
			item.Caption,
			item.Title,
			item.Description,
		} {
			if text := strings.TrimSpace(candidate); text != "" && !isFormatName(text) {
				return text
			}
		}
	}
	return ""
}

func isFormatName(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "mp4", "jpg", "jpeg", "png", "webp", "heic", "video", "photo", "image", "gif", "mov", "avi", "mp3", "m4a", "flac", "ogg", "webm":
		return true
	default:
		return false
	}
}

func ParseEmbedGQL(body []byte) (*Media, error) {
	match := embedPattern.FindSubmatch(body)
	if len(match) < 2 {
		return nil, fmt.Errorf("gql json not found")
	}
	jsonData := match[1]

	var data map[string]any
	if err := json5.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	igCtx := util.TraverseJSON(data, "contextJSON")
	if igCtx == nil {
		return nil, fmt.Errorf("contextJSON not found")
	}
	var ctxJSON ContextJSON
	switch v := igCtx.(type) {
	case string:
		if err := json5.Unmarshal([]byte(v), &ctxJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal contextJSON: %w", err)
		}
	default:
		return nil, fmt.Errorf("unexpected type for contextJSON: %T", v)
	}
	if ctxJSON.GqlData == nil {
		return nil, fmt.Errorf("gql_data not found")
	}
	if ctxJSON.GqlData.ShortcodeMedia == nil {
		return nil, fmt.Errorf("shortcode_media not found")
	}
	media := ctxJSON.GqlData.ShortcodeMedia
	if mediaCaption(media) == "" {
		media.Title = contextCaption(&ctxJSON)
	}
	return media, nil
}

func IGramBodyFromURL(contentURL string) (io.Reader, error) {
	return igramBuildPayload(map[string]string{
		"target_url": contentURL,
	})
}

func IGramBodyFromParams(params map[string]string) (io.Reader, error) {
	return igramBuildPayload(params)
}

func igramBuildPayload(urlParams map[string]string) (io.Reader, error) {
	nowMs := time.Now().UnixMilli()
	serverMs := getIGramServerTime()

	drift := serverMs - nowMs
	var correction int64
	if drift >= 60000 || drift <= -60000 {
		correction = drift
	}
	ts := nowMs + correction

	// partial payload fields that get signed
	partial := map[string]any{
		"_sc": 0,
		"_ef": 0,
		"_df": 0,
	}
	for k, v := range urlParams {
		partial[k] = v
	}

	sig, err := igramSign(partial, ts)
	if err != nil {
		return nil, err
	}

	// assemble final payload
	final := make(map[string]any, len(partial)+5)
	for k, v := range partial {
		final[k] = v
	}
	final["ts"] = ts
	final["_ts"] = igramStaticTS
	final["_tsc"] = correction
	final["_sv"] = 2
	final["_s"] = sig

	jsonBytes, err := sonic.ConfigFastest.Marshal(final)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	return strings.NewReader(string(jsonBytes)), nil
}

func igramSign(partial map[string]any, ts int64) (string, error) {
	// sonic.ConfigStd sorts map keys alphabetically, matching
	// the signing: JSON.stringify(sorted_partial) + String(ts)
	jsonBytes, err := sonic.ConfigStd.Marshal(partial)
	if err != nil {
		return "", fmt.Errorf("failed to marshal partial payload: %w", err)
	}

	data := string(jsonBytes) + strconv.FormatInt(ts, 10)

	keyBytes, err := hex.DecodeString(igramHMACKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode HMAC key: %w", err)
	}

	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func getIGramServerTime() int64 {
	apiURL := fmt.Sprintf("https://%s/msec", igramAPIBase)
	resp, err := http.Get(apiURL)
	if err != nil {
		return time.Now().UnixMilli()
	}
	defer resp.Body.Close()

	var result struct {
		Msec float64 `json:"msec"`
	}
	decoder := sonic.ConfigFastest.NewDecoder(resp.Body)
	if err := decoder.Decode(&result); err != nil {
		return time.Now().UnixMilli()
	}
	return int64(result.Msec * 1000)
}

func ParseIGramResponse(body []byte) (*IGramResponse, error) {
	// try to unmarshal as a single IGramMedia and then as a slice
	var media IGramMedia

	if err := sonic.ConfigFastest.Unmarshal(body, &media); err != nil {
		// try with slice
		var mediaList []*IGramMedia
		if err := sonic.ConfigFastest.Unmarshal(body, &mediaList); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &IGramResponse{
			Items: mediaList,
		}, nil
	}
	if media.Success != nil && !(*media.Success) {
		return nil, util.ErrUnavailable
	}
	return &IGramResponse{
		Items: []*IGramMedia{&media},
	}, nil
}

func GetCDNURL(contentURL string) (string, error) {
	contentURL = strings.TrimSpace(contentURL)
	if contentURL == "" {
		return "", fmt.Errorf("empty igram URL")
	}
	parsedURL, err := url.Parse(contentURL)
	if err != nil {
		return "", fmt.Errorf("can't parse igram URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("unsupported igram URL scheme: %s", parsedURL.Scheme)
	}
	queryParams, err := url.ParseQuery(parsedURL.RawQuery)
	if err != nil {
		return "", fmt.Errorf("can't unescape igram URL: %w", err)
	}
	cdnURL := queryParams.Get("uri")
	if cdnURL == "" {
		if parsedURL.Host == "" {
			return "", fmt.Errorf("igram URL has no host")
		}
		return contentURL, nil
	}
	if !isHTTPURL(cdnURL) {
		return "", fmt.Errorf("invalid igram CDN URL")
	}
	return cdnURL, nil
}

func instagramDownloadSettings() *models.DownloadSettings {
	return &models.DownloadSettings{
		Headers:     instagramDownloadHeaders(),
		Impersonate: true,
	}
}

func instagramDownloadHeaders() map[string]string {
	headers := make(map[string]string, len(webHeaders)+2)
	for key, value := range webHeaders {
		headers[key] = value
	}
	headers["Referer"] = "https://www.instagram.com/"
	headers["Origin"] = "https://www.instagram.com"
	return headers
}

func isHTTPURL(rawURL string) bool {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	return (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") &&
		parsedURL.Host != ""
}

func GetGQLData(ctx *models.ExtractorContext) (*GraphQLData, error) {
	graphHeaders, body, err := BuildGQLData()
	if err != nil {
		return nil, fmt.Errorf("failed to build GQL data: %w", err)
	}
	if len(ctx.HTTPClient.Cookies) > 0 {
		delete(graphHeaders, "cookie")
	}
	formData := url.Values{}
	for key, value := range body {
		formData.Set(key, value)
	}
	formData.Set("fb_api_caller_class", "RelayModern")
	formData.Set("fb_api_req_friendly_name", polarisAction)
	variables := map[string]any{
		"shortcode":               ctx.ContentID,
		"fetch_tagged_user_count": nil,
		"hoisted_comment_id":      nil,
		"hoisted_reply_id":        nil,
	}
	variablesJSON, err := sonic.ConfigFastest.Marshal(variables)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal variables: %w", err)
	}
	formData.Set("variables", string(variablesJSON))
	formData.Set("server_timestamps", "true")
	formData.Set("doc_id", "8845758582119845") // idk what this is

	for key, value := range webHeaders {
		graphHeaders[key] = value
	}
	resp, err := ctx.Fetch(
		http.MethodPost,
		graphQLEndpoint,
		&networking.RequestParams{
			Headers: graphHeaders,
			Body:    strings.NewReader(formData.Encode()),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.WriteFile("iggql_api_response", resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid response code: %s", resp.Status)
	}
	var response GraphQLResponse
	decoder := sonic.ConfigFastest.NewDecoder(resp.Body)
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if response.Data == nil {
		return nil, fmt.Errorf("data is nil")
	}
	if response.Status != "ok" {
		return nil, fmt.Errorf("status is not ok: %s", response.Status)
	}
	if response.Data.ShortcodeMedia == nil {
		return nil, fmt.Errorf("shortcode_media is nil")
	}
	return response.Data, nil
}

func BuildGQLData() (map[string]string, map[string]string, error) {
	const (
		domain                = "www"
		requestID             = "b"
		clientCapabilityGrade = "EXCELLENT"
		sessionInternalID     = "7436540909012459023"
		apiVersion            = "1"
		rolloutHash           = "1019933358"
		appID                 = "936619743392459"
		bloksVersionID        = "6309c8d03d8a3f47a1658ba38b304a3f837142ef5f637ebf1f8f52d4b802951e"
		asbdID                = "129477"
		hiddenState           = "20126.HYP:instagram_web_pkg.2.1...0"
		loggedIn              = "0"
		cometRequestID        = "7"
		appVersion            = "0"
		pixelRatio            = "2"
		buildType             = "trunk"
	)
	session := "::" + util.RandomAlphaString(6)
	sessionData := util.RandomBase64(8)
	csrfToken := util.RandomBase64(32)
	deviceID := util.RandomBase64(24)
	machineID := util.RandomBase64(24)
	dynamicFlags := util.RandomBase64(154)
	clientSessionRnd := util.RandomBase64(154)
	jazoestBig, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate jazoest: %w", err)
	}
	jazoest := strconv.FormatInt(jazoestBig.Int64()+1, 10)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	cookies := []string{
		"csrftoken=" + csrfToken,
		"ig_did=" + deviceID,
		"wd=1280x720",
		"dpr=2",
		"mid=" + machineID,
		"ig_nrcb=1",
	}
	headers := map[string]string{
		"x-ig-app-id":        appID,
		"X-FB-LSD":           sessionData,
		"X-CSRFToken":        csrfToken,
		"X-Bloks-Version-Id": bloksVersionID,
		"x-asbd-id":          asbdID,
		"cookie":             strings.Join(cookies, "; "),
		"Content-Type":       "application/x-www-form-urlencoded",
		"X-FB-Friendly-Name": polarisAction,
	}
	body := map[string]string{
		"__d":         domain,
		"__a":         apiVersion,
		"__s":         session,
		"__hs":        hiddenState,
		"__req":       requestID,
		"__ccg":       clientCapabilityGrade,
		"__rev":       rolloutHash,
		"__hsi":       sessionInternalID,
		"__dyn":       dynamicFlags,
		"__csr":       clientSessionRnd,
		"__user":      loggedIn,
		"__comet_req": cometRequestID,
		"libav":       appVersion,
		"dpr":         pixelRatio,
		"lsd":         sessionData,
		"jazoest":     jazoest,
		"__spin_r":    rolloutHash,
		"__spin_b":    buildType,
		"__spin_t":    timestamp,
	}
	return headers, body, nil
}

func GetBestCandidate(candidates []*Candidates) *Candidates {
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0]
	for _, candidate := range candidates {
		if candidate.Width > best.Width {
			best = candidate
		}
	}
	return best
}

func GetBestVideoVersion(versions []*VideoVersions) *VideoVersions {
	if len(versions) == 0 {
		return nil
	}
	best := versions[0]
	for _, version := range versions {
		if version.Width > best.Width {
			best = version
		}
	}
	return best
}
