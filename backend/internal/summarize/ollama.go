package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yourname/go-news-llm/backend/internal/textutil"
)

type Ollama struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func (o Ollama) Summarize(ctx context.Context, title string, content string) (string, error) {
	if o.BaseURL == "" {
		return "", errors.New("ollama base url not set")
	}
	model := o.Model
	if model == "" {
		model = "llama3.1"
	}
	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}

	title = textutil.PlainText(title)
	content = textutil.PlainText(content)
	if content == "" {
		content = title
	}
	prompt := fmt.Sprintf(
		"Summarize in 1-2 short sentences. Plain text only, no HTML.\n\nTitle: %s\n\nContent:\n%s\n",
		title,
		trimForPrompt(content, 3500),
	)

	body, _ := json.Marshal(map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(o.BaseURL, "/")+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama http %d", resp.StatusCode)
	}
	var decoded struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", err
	}
	s := textutil.PlainText(decoded.Response)
	if s == "" {
		return "", errors.New("empty ollama response")
	}
	return s, nil
}

func trimForPrompt(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}

