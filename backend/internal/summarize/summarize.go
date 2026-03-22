package summarize

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/yourname/go-news-llm/backend/internal/textutil"
)

type Summarizer interface {
	Summarize(ctx context.Context, title string, content string) (string, error)
}

// Fallback is a zero-cost summarizer when no local LLM is configured.
type Fallback struct{}

func (Fallback) Summarize(_ context.Context, title string, content string) (string, error) {
	title = textutil.PlainText(title)
	content = textutil.PlainText(content)
	if content == "" {
		if title != "" {
			return title, nil
		}
		return "No summary available.", nil
	}
	// Very small, safe extractive summary.
	s := content
	if idx := strings.IndexAny(s, "\n"); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		if title != "" {
			return title, nil
		}
		return "No summary available.", nil
	}
	const max = 260
	if utf8.RuneCountInString(s) > max {
		r := []rune(s)
		s = strings.TrimSpace(string(r[:max])) + "…"
	}
	return s, nil
}

