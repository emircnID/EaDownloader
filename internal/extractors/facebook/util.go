package facebook

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"eadownloader/internal/logger"
	"eadownloader/internal/models"
	"eadownloader/internal/networking"
)

var webHeaders = map[string]string{
	"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	"Accept-Language":           "en-US,en;q=0.5",
	"Sec-Fetch-Dest":            "document",
	"Sec-Fetch-Mode":            "navigate",
	"Sec-Fetch-Site":            "none",
	"Sec-Fetch-User":            "?1",
	"Upgrade-Insecure-Requests": "1",
}

var (
	hdURLPatterns = []*regexp.Regexp{
		regexp.MustCompile(
			`"progressive_url"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"\s*,\s*"failure_reason"\s*:\s*[^,]+\s*,\s*"metadata"\s*:\s*\{\s*"quality"\s*:\s*"HD"\s*\}`,
		),
		regexp.MustCompile(`"browser_native_hd_url"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`),
		regexp.MustCompile(`"playable_url_quality_hd"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`),
		regexp.MustCompile(`"hd_src"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`),
	}
	sdURLPatterns = []*regexp.Regexp{
		regexp.MustCompile(
			`"progressive_url"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"\s*,\s*"failure_reason"\s*:\s*[^,]+\s*,\s*"metadata"\s*:\s*\{\s*"quality"\s*:\s*"SD"\s*\}`,
		),
		regexp.MustCompile(`"browser_native_sd_url"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`),
		regexp.MustCompile(`"playable_url"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`),
		regexp.MustCompile(`"sd_src"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`),
	}
	titlePattern = regexp.MustCompile(
		`"title"\s*:\s*\{\s*"text"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`,
	)
	ogVideoPattern   = regexp.MustCompile(`<meta\s+property=["']og:video(?::secure_url)?["']\s+content=["']([^"']+)["']`)
	imageURLPatterns = []*regexp.Regexp{
		regexp.MustCompile(`<meta\s+property=["']og:image(?::secure_url)?["']\s+content=["']([^"']+)["']`),
		regexp.MustCompile(`<meta\s+content=["']([^"']+)["']\s+property=["']og:image(?::secure_url)?["']`),
		regexp.MustCompile(`"image"\s*:\s*\{[^{}]{0,800}"uri"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`),
		regexp.MustCompile(`"image"\s*:\s*\{[^{}]{0,800}"url"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`),
		regexp.MustCompile(`"photo_image"\s*:\s*\{[^{}]{0,800}"uri"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`),
		regexp.MustCompile(`"photo_image"\s*:\s*\{[^{}]{0,800}"url"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`),
		regexp.MustCompile(`"preferred_thumbnail"\s*:\s*\{[^{}]{0,999}"image"\s*:\s*\{[^{}]{0,800}"uri"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`),
		regexp.MustCompile(`"preferred_thumbnail"\s*:\s*\{[^{}]{0,999}"image"\s*:\s*\{[^{}]{0,800}"url"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`),
	}
)

func GetVideoData(ctx *models.ExtractorContext) (*VideoData, error) {
	contentURL := strings.Replace(ctx.ContentURL, "m.facebook.com", "www.facebook.com", 1)
	contentURL = strings.Replace(contentURL, "mbasic.facebook.com", "www.facebook.com", 1)

	// convert watch URLs to reel permalink,
	// /watch/?v=XXX pages return wrong video data when scraped
	if strings.Contains(contentURL, "/watch") && ctx.ContentID != "" {
		contentURL = "https://www.facebook.com/reel/" + ctx.ContentID
	}

	resp, err := ctx.Fetch(
		http.MethodGet,
		contentURL,
		&networking.RequestParams{
			Headers: webHeaders,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.WriteFile("fb_response", resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get page: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return parseVideoFromBody(body, ctx.ContentID)
}

func parseVideoFromBody(body []byte, videoID string) (*VideoData, error) {
	data := &VideoData{}

	// find the section belonging to the requested video
	section := findVideoSection(body, videoID)
	if section == nil {
		// fall back to full body for reel/post pages with a single video
		section = body
	}

	data.HDURL = firstMatchedURL(section, hdURLPatterns)
	data.SDURL = firstMatchedURL(section, sdURLPatterns)
	if data.HDURL == "" {
		data.HDURL = firstMatchedURL(body, hdURLPatterns)
	}
	if data.SDURL == "" {
		data.SDURL = firstMatchedURL(body, sdURLPatterns)
	}
	if data.SDURL == "" {
		data.SDURL = firstMatchedURL(body, []*regexp.Regexp{ogVideoPattern})
	}
	data.ImageURLs = matchedImageURLs(section)
	if len(data.ImageURLs) == 0 {
		data.ImageURLs = firstMatchedImageURL(body)
	}
	// title can be anywhere in the page
	if match := titlePattern.FindSubmatch(body); len(match) >= 2 {
		data.Title = unescapeFacebookString(string(match[1]))
	}

	if data.HDURL == "" && data.SDURL == "" && len(data.ImageURLs) == 0 {
		return nil, fmt.Errorf("no media URLs found in page")
	}

	return data, nil
}

func firstMatchedURL(body []byte, patterns []*regexp.Regexp) string {
	for _, pattern := range patterns {
		match := pattern.FindSubmatch(body)
		if len(match) >= 2 {
			return unescapeFacebookURL(string(match[1]))
		}
	}
	return ""
}

func matchedImageURLs(sections ...[]byte) []string {
	seen := make(map[string]struct{})
	imageURLs := make([]string, 0)
	for _, section := range sections {
		for _, pattern := range imageURLPatterns {
			for _, match := range pattern.FindAllSubmatch(section, -1) {
				if len(match) < 2 {
					continue
				}
				imageURL := unescapeFacebookURL(string(match[1]))
				if !isFacebookImageURL(imageURL) {
					continue
				}
				if _, ok := seen[imageURL]; ok {
					continue
				}
				seen[imageURL] = struct{}{}
				imageURLs = append(imageURLs, imageURL)
			}
		}
	}
	return imageURLs
}

func firstMatchedImageURL(body []byte) []string {
	match := imageURLPatterns[0].FindSubmatch(body)
	if len(match) < 2 {
		return nil
	}
	imageURL := unescapeFacebookURL(string(match[1]))
	if !isFacebookImageURL(imageURL) {
		return nil
	}
	return []string{imageURL}
}

func isFacebookImageURL(imageURL string) bool {
	parsedURL, err := url.Parse(imageURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsedURL.Hostname())
	return strings.Contains(host, "fbcdn.net") ||
		strings.Contains(host, "fbsbx.com")
}

// findVideoSection returns the slice of body containing the video delivery
// data for the given videoID, anchored by dash_mpd_debug.mpd?v=VIDEO_ID
// and bounded by the closing "id":"VIDEO_ID".
func findVideoSection(body []byte, videoID string) []byte {
	if videoID == "" {
		return nil
	}

	anchor := []byte("dash_mpd_debug.mpd?v=" + videoID)
	start := bytes.Index(body, anchor)
	if start == -1 {
		return nil
	}

	remaining := body[start:]

	// look for "id":"VIDEO_ID" which closes the videoDeliveryResponseResult block
	endMarker := []byte(`"id":"` + videoID + `"`)
	endIdx := bytes.Index(remaining, endMarker)
	if endIdx > 0 {
		return remaining[:endIdx+len(endMarker)]
	}

	// fallback: take a generous window
	maxLen := 20000
	if maxLen > len(remaining) {
		maxLen = len(remaining)
	}
	return remaining[:maxLen]
}

func unescapeFacebookURL(s string) string {
	return unescapeFacebookString(s)
}

func unescapeFacebookString(s string) string {
	s = strings.ReplaceAll(s, `\/`, "/")
	if unquoted, err := strconv.Unquote(`"` + s + `"`); err == nil {
		s = unquoted
	} else {
		s = unescapeUnicode(s)
	}
	return html.UnescapeString(s)
}

func unescapeUnicode(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		if i+5 < len(s) && s[i] == '\\' && s[i+1] == 'u' {
			var r rune
			valid := true
			for j := 2; j < 6; j++ {
				r <<= 4
				c := s[i+j]
				switch {
				case c >= '0' && c <= '9':
					r |= rune(c - '0')
				case c >= 'a' && c <= 'f':
					r |= rune(c - 'a' + 10)
				case c >= 'A' && c <= 'F':
					r |= rune(c - 'A' + 10)
				default:
					valid = false
				}
			}
			if valid && utf8.ValidRune(r) {
				b.WriteRune(r)
				i += 6
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
