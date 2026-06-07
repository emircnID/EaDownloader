package textutil

import "strings"

func SanitizeTelegramText(text string) string {
	text = strings.ToValidUTF8(text, "")
	text = strings.ReplaceAll(text, "\x00", "")
	return strings.TrimSpace(text)
}

func TruncateTelegramText(text string, limit int) string {
	text = SanitizeTelegramText(text)
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}
