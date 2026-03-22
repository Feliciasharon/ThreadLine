package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/yourname/go-news-llm/backend/internal/bus"
	"github.com/yourname/go-news-llm/backend/internal/db"
	"github.com/yourname/go-news-llm/backend/internal/summarize"
	"github.com/yourname/go-news-llm/backend/internal/worker"
)

func main() {
	logger := log.Default()

	d, err := db.OpenFromEnv()
	if err != nil {
		logger.Fatalf("db open: %v", err)
	}
	defer d.Close()

	var bc *bus.Client
	if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
		c, err := bus.Connect(natsURL)
		if err != nil {
			logger.Printf("nats disabled (connect failed): %v", err)
		} else {
			bc = c
			defer bc.Close()
		}
	} else {
		logger.Printf("nats disabled (NATS_URL not set)")
	}

	var sum summarize.Summarizer = summarize.Fallback{}
	if key := strings.TrimSpace(os.Getenv("GEMINI_API_KEY")); key != "" {
		sum = summarize.Gemini{
			APIKey: key,
			Model:  env("GEMINI_MODEL", "gemini-2.0-flash"),
		}
		logger.Printf("gemini enabled (%s)", env("GEMINI_MODEL", "gemini-2.0-flash"))
	} else if base := strings.TrimSpace(os.Getenv("OLLAMA_BASE_URL")); base != "" {
		sum = summarize.Ollama{
			BaseURL: base,
			Model:   env("OLLAMA_MODEL", "llama3.1"),
		}
		logger.Printf("ollama enabled: %s (%s)", base, env("OLLAMA_MODEL", "llama3.1"))
	} else {
		logger.Printf("llm disabled (set GEMINI_API_KEY or OLLAMA_BASE_URL to enable)")
	}

	w := &worker.Worker{
		DB:         d,
		NC:         func() *nats.Conn { if bc == nil { return nil }; return bc.Conn }(),
		Summarizer: sum,
		Logger:     logger,
	}
	// Space out Gemini calls slightly to reduce 429 / empty responses on free tier.
	if strings.TrimSpace(os.Getenv("GEMINI_API_KEY")) != "" {
		w.SummarizePause = 350 * time.Millisecond
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := w.Run(ctx); err != nil && err != context.Canceled {
		logger.Fatalf("worker: %v", err)
	}
}

func env(k, def string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	return v
}

