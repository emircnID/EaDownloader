package models

import "net/http"

type DownloadSettings struct {
	NumConnections int
	ChunkSize      int64
	Headers        map[string]string
	Cookies        []*http.Cookie
	DecryptionKey  *DecryptionKey
	Retries        int
	SkipRemux      bool
	SkipThumbnail  bool
	CobaltURL      string
	CobaltQuality  string
	CobaltAudio    bool
	YtDLPURL       string
	YtDLPFormat    string
	YtDLPSort      string
	YtDLPCookieJar string
	YtDLPArgs      string
	YtDLPAudio     bool
}
