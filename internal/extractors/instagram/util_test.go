package instagram

import (
	"testing"

	"eadownloader/internal/models"
)

func TestMediaCaptionUsesTitleFallback(t *testing.T) {
	got := mediaCaption(&Media{Title: "fallback title"})
	if got != "fallback title" {
		t.Fatalf("expected title fallback, got %q", got)
	}
}

func TestMediaCaptionPrefersEdgeCaption(t *testing.T) {
	got := mediaCaption(&Media{
		Title: "fallback title",
		EdgeMediaToCaption: &EdgeMediaToCaption{
			Edges: []*Edges{{Node: &Node{Text: "post caption"}}},
		},
	})
	if got != "post caption" {
		t.Fatalf("expected edge caption, got %q", got)
	}
}

func TestContextCaption(t *testing.T) {
	got := contextCaption(&ContextJSON{
		Context: &Context{
			Title:   "title",
			Caption: "caption",
		},
	})
	if got != "caption" {
		t.Fatalf("expected context caption, got %q", got)
	}
}

func TestIGramCaption(t *testing.T) {
	got := igramCaption(&IGramResponse{
		Items: []*IGramMedia{
			{
				URL: []*IGramMediaURL{{Name: "media name"}},
			},
			{
				Caption: "post caption",
			},
		},
	})
	if got != "post caption" {
		t.Fatalf("expected first available caption candidate, got %q", got)
	}
}

func TestIGramCaptionIgnoresFormatNames(t *testing.T) {
	got := igramCaption(&IGramResponse{
		Items: []*IGramMedia{
			{
				Caption: "mp4",
				Title:   "video",
			},
			{
				Caption: "My real post caption",
			},
		},
	})
	if got != "My real post caption" {
		t.Fatalf("expected real post caption, got %q", got)
	}
}

func TestGetCDNURLRejectsEmptyURL(t *testing.T) {
	if _, err := GetCDNURL(""); err == nil {
		t.Fatal("expected empty URL to fail")
	}
}

func TestGetCDNURLReturnsDirectURLWhenNoWrappedURI(t *testing.T) {
	const directURL = "https://cdn.example.com/video.mp4"
	got, err := GetCDNURL(directURL)
	if err != nil {
		t.Fatalf("expected direct URL to parse: %v", err)
	}
	if got != directURL {
		t.Fatalf("expected %q, got %q", directURL, got)
	}
}

func TestGetCDNURLExtractsWrappedURI(t *testing.T) {
	const cdnURL = "https://cdn.example.com/video.mp4"
	got, err := GetCDNURL("https://igram.example/download?uri=https%3A%2F%2Fcdn.example.com%2Fvideo.mp4")
	if err != nil {
		t.Fatalf("expected wrapped URL to parse: %v", err)
	}
	if got != cdnURL {
		t.Fatalf("expected %q, got %q", cdnURL, got)
	}
}

func TestParseGQLMediaRejectsEmptyDownloadURLs(t *testing.T) {
	ctx := &models.ExtractorContext{
		Extractor:  Extractor,
		ContentID:  "shortcode",
		ContentURL: "https://www.instagram.com/p/shortcode/",
	}

	_, err := ParseGQLMedia(ctx, &Media{
		Typename: "GraphVideo",
		VideoURL: "",
	})
	if err == nil {
		t.Fatal("expected media with no downloadable URL to fail")
	}
}
