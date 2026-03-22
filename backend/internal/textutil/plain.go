package textutil

import (
	"html"
	"regexp"
	"strings"
)

var (
	scriptStyle = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>`)
	comments    = regexp.MustCompile(`(?s)<!--.*?-->`)
	tags        = regexp.MustCompile(`<[^>]+>`)
)

// PlainText strips HTML/XML-ish markup from RSS descriptions so LLMs and UIs get readable text.
func PlainText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = scriptStyle.ReplaceAllString(s, " ")
	s = comments.ReplaceAllString(s, " ")
	s = tags.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}
