package cobalt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"eadownloader/internal/config"
)

type Request struct {
	URL                   string `json:"url"`
	AudioBitrate          string `json:"audioBitrate,omitempty"`
	AudioFormat           string `json:"audioFormat,omitempty"`
	DownloadMode          string `json:"downloadMode,omitempty"`
	FilenameStyle         string `json:"filenameStyle,omitempty"`
	VideoQuality          string `json:"videoQuality,omitempty"`
	DisableMetadata       bool   `json:"disableMetadata,omitempty"`
	AlwaysProxy           bool   `json:"alwaysProxy,omitempty"`
	LocalProcessing       string `json:"localProcessing,omitempty"`
	YoutubeVideoCodec     string `json:"youtubeVideoCodec,omitempty"`
	YoutubeVideoContainer string `json:"youtubeVideoContainer,omitempty"`
	YoutubeBetterAudio    bool   `json:"youtubeBetterAudio,omitempty"`
	YoutubeHLS            bool   `json:"youtubeHLS,omitempty"`
}

type Response struct {
	Status   string       `json:"status"`
	URL      string       `json:"url,omitempty"`
	Filename string       `json:"filename,omitempty"`
	Picker   []PickerItem `json:"picker,omitempty"`
	Error    *Error       `json:"error,omitempty"`
}

type PickerItem struct {
	Type  string `json:"type"`
	URL   string `json:"url"`
	Label string `json:"label,omitempty"`
	Thumb string `json:"thumb,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Context string `json:"context,omitempty"`
}

func GetMedia(url string) (*Response, error) {
	return GetMediaWithOptions(Request{
		URL:             url,
		DisableMetadata: true,
	})
}

func GetMediaWithOptions(reqData Request) (*Response, error) {
	if reqData.VideoQuality == "" {
		reqData.VideoQuality = "1080"
	}
	if reqData.FilenameStyle == "" {
		reqData.FilenameStyle = "basic"
	}
	if reqData.YoutubeVideoCodec == "" {
		reqData.YoutubeVideoCodec = "h264"
	}
	if reqData.YoutubeVideoContainer == "" {
		reqData.YoutubeVideoContainer = "mp4"
	}
	if reqData.DownloadMode == "" {
		reqData.DownloadMode = "auto"
	}
	if reqData.AudioFormat == "" {
		reqData.AudioFormat = "mp3"
	}
	if reqData.AudioBitrate == "" {
		reqData.AudioBitrate = "128"
	}

	body, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, cobaltAPIURL(), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to cobalt api failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var cobaltResp Response
	if err := json.Unmarshal(respBody, &cobaltResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if cobaltResp.Status == "error" {
		if cobaltResp.Error != nil {
			return nil, fmt.Errorf("cobalt api error: %s", cobaltResp.Error.Code)
		}
		return nil, fmt.Errorf("cobalt api returned an unknown error")
	}

	return &cobaltResp, nil
}

func cobaltAPIURL() string {
	if config.Env.CobaltAPIURL != "" {
		return config.Env.CobaltAPIURL
	}
	return "http://cobalt-api:9000"
}
