package facebook

// VideoData holds extracted media information from Facebook page HTML.
type VideoData struct {
	HDURL     string
	SDURL     string
	ImageURLs []string
	Title     string
	Width     int32
	Height    int32
}
