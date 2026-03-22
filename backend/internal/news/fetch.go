package news

import (
	"context"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

type Item struct {
	Title       string
	URL         string
	Source      string
	PublishedAt time.Time
	Content     string
}

type RSSSource struct {
	Name string
	URL  string
}

var DefaultSources = []RSSSource{
	{Name: "BBC World", URL: "https://feeds.bbci.co.uk/news/world/rss.xml"},
	{Name: "The Guardian World", URL: "https://www.theguardian.com/world/rss"},
	{Name: "NYT World", URL: "https://rss.nytimes.com/services/xml/rss/nyt/World.xml"},
	{Name: "NPR World", URL: "https://feeds.npr.org/1004/rss.xml"},
}

func FetchRSS(ctx context.Context, client *http.Client, src RSSSource, maxItems int) ([]Item, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	fp := gofeed.NewParser()
	fp.Client = client

	feed, err := fp.ParseURLWithContext(src.URL, ctx)
	if err != nil {
		return nil, err
	}

	if maxItems <= 0 || maxItems > 100 {
		maxItems = 30
	}
	out := make([]Item, 0, maxItems)
	for _, it := range feed.Items {
		if it == nil || it.Link == "" || it.Title == "" {
			continue
		}
		pub := time.Now().UTC()
		if it.PublishedParsed != nil {
			pub = it.PublishedParsed.UTC()
		} else if it.UpdatedParsed != nil {
			pub = it.UpdatedParsed.UTC()
		}
		content := ""
		if it.Content != "" {
			content = it.Content
		} else if it.Description != "" {
			content = it.Description
		}
		out = append(out, Item{
			Title:       it.Title,
			URL:         it.Link,
			Source:      src.Name,
			PublishedAt: pub,
			Content:     content,
		})
		if len(out) >= maxItems {
			break
		}
	}
	return out, nil
}

func FetchAll(ctx context.Context, sources []RSSSource) ([]Item, error) {
	if len(sources) == 0 {
		sources = DefaultSources
	}
	client := &http.Client{Timeout: 12 * time.Second}

	out := make([]Item, 0, 120)
	for _, s := range sources {
		items, err := FetchRSS(ctx, client, s, 30)
		if err != nil {
			// Best-effort: skip sources that fail.
			continue
		}
		out = append(out, items...)
	}
	return out, nil
}

