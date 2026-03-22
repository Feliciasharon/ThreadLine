package worker

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/yourname/go-news-llm/backend/internal/db"
	"github.com/yourname/go-news-llm/backend/internal/news"
	"github.com/yourname/go-news-llm/backend/internal/summarize"
	"github.com/yourname/go-news-llm/backend/internal/textutil"
)

type Worker struct {
	DB             *db.DB
	NC             *nats.Conn
	Summarizer     summarize.Summarizer
	Logger         *log.Logger
	SummarizePause time.Duration // optional delay between items (helps Gemini rate limits)
}

const (
	SubjectRefresh = "news.refresh"
	SubjectNew     = "news.new"
)

func (w *Worker) Run(ctx context.Context) error {
	if w.Logger == nil {
		w.Logger = log.Default()
	}
	if w.Summarizer == nil {
		w.Summarizer = summarize.Fallback{}
	}

	var sub *nats.Subscription
	if w.NC != nil {
		s, err := w.NC.SubscribeSync(SubjectRefresh)
		if err != nil {
			return err
		}
		sub = s
		defer func() { _ = sub.Unsubscribe() }()
	}

	// Do an initial refresh on boot.
	if _, err := w.refreshOnce(ctx); err != nil {
		w.Logger.Printf("initial refresh failed: %v", err)
	}

	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := w.refreshOnce(ctx); err != nil {
				w.Logger.Printf("scheduled refresh failed: %v", err)
			}
		default:
			if sub == nil {
				time.Sleep(250 * time.Millisecond)
				continue
			}
			msg, err := sub.NextMsg(250 * time.Millisecond)
			if err == nats.ErrTimeout {
				continue
			}
			if err != nil {
				return err
			}
			n, err := w.refreshOnce(ctx)
			if err != nil {
				_ = msg.Respond([]byte("error"))
				continue
			}
			_ = msg.Respond([]byte("ok:" + itoa(n)))
		}
	}
}

func (w *Worker) refreshOnce(ctx context.Context) (int, error) {
	items, err := news.FetchAll(ctx, nil)
	if err != nil {
		return 0, err
	}

	out := make([]db.NewsItem, 0, len(items))
	var sumErrs int
	for i, it := range items {
		if i > 0 && w.SummarizePause > 0 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(w.SummarizePause):
			}
		}

		title := textutil.PlainText(it.Title)
		if title == "" {
			title = strings.TrimSpace(it.Title)
		}
		body := textutil.PlainText(it.Content)
		if body == "" {
			body = title
		}

		sum, err := w.Summarizer.Summarize(ctx, title, body)
		quotaExceeded := false
		if err != nil {
			sumErrs++
			low := strings.ToLower(err.Error())
			if strings.Contains(low, "quota exceeded") || strings.Contains(low, "resource_exhausted") {
				quotaExceeded = true
			}
			if w.Logger != nil && sumErrs <= 5 {
				w.Logger.Printf("summarize: %v", err)
			}
		}
		if err != nil || sum == "" {
			sum, _ = summarize.Fallback{}.Summarize(ctx, title, body)
		}
		sum = textutil.PlainText(sum)
		if sum == "" {
			sum = title
		}
		if quotaExceeded {
			sum = sum
		}

		out = append(out, db.NewsItem{
			ID:          stableID(it.URL),
			Title:       it.Title,
			URL:         it.URL,
			Source:      it.Source,
			PublishedAt: it.PublishedAt,
			Summary:     sum,
		})
	}
	if sumErrs > 0 && w.Logger != nil {
		w.Logger.Printf("summarize: %d item(s) needed fallback (see errors above)", sumErrs)
	}

	n, err := w.DB.UpsertNews(ctx, out)
	if err != nil {
		return 0, err
	}

	// Publish latest items best-effort (frontends can treat as "live").
	if w.NC != nil {
		for _, it := range out {
			b, _ := json.Marshal(it)
			_ = w.NC.Publish(SubjectNew, b)
		}
	}

	return n, nil
}

func stableID(url string) string {
	h := sha1.Sum([]byte(url))
	return hex.EncodeToString(h[:])
}

// itoa avoids pulling in strconv in the hot path.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [32]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
