package instagram

import "testing"

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
