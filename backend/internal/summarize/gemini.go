package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yourname/go-news-llm/backend/internal/textutil"
)

// Gemini uses the Google Generative Language API (AI Studio key).
// Docs: https://ai.google.dev/
type Gemini struct {
	APIKey string
	Model  string // e.g. "gemini-2.0-flash"
	Client *http.Client
}

func (g Gemini) Summarize(ctx context.Context, title string, content string) (string, error) {
	key := strings.TrimSpace(g.APIKey)
	if key == "" {
		return "", errors.New("gemini api key not set")
	}
	model := strings.TrimSpace(g.Model)
	if model == "" {
		model = "gemini-2.0-flash"
	}
	client := g.Client
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}

	title = textutil.PlainText(title)
	content = textutil.PlainText(content)
	if content == "" {
		content = title
	}

	prompt := fmt.Sprintf(
		"You are a news assistant. Summarize in 1-2 short sentences. Output plain text only — no HTML, no markdown, no bullet lists.\n\nTitle: %s\n\nArticle text:\n%s\n",
		title,
		trimForPrompt(content, 6000),
	)

	reqBody, _ := json.Marshal(map[string]any{
		"contents": []any{
			map[string]any{
				"role": "user",
				"parts": []any{
					map[string]any{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":      0.2,
			"maxOutputTokens":  120,
			"topP":             0.9,
			"responseMimeType": "text/plain",
		},
	})

	u := "https://generativelanguage.googleapis.com/v1beta/models/" + url.PathEscape(model) + ":generateContent?key=" + url.QueryEscape(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &apiErr)
		msg := strings.TrimSpace(apiErr.Error.Message)
		if msg == "" {
			msg = string(bytes.TrimSpace(body))
			if len(msg) > 400 {
				msg = msg[:400] + "…"
			}
		}
		if msg != "" {
			return "", fmt.Errorf("gemini http %d: %s", resp.StatusCode, msg)
		}
		return "", fmt.Errorf("gemini http %d", resp.StatusCode)
	}

	var decoded struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", err
	}
	if len(decoded.Candidates) == 0 || len(decoded.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("gemini: empty candidates")
	}
	s := textutil.PlainText(decoded.Candidates[0].Content.Parts[0].Text)
	if s == "" {
		return "", errors.New("gemini: empty text")
	}
	return s, nil
}

