package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/yourname/go-news-llm/backend/internal/bus"
	"github.com/yourname/go-news-llm/backend/internal/db"
	"github.com/yourname/go-news-llm/backend/internal/events"
	graphqlsrv "github.com/yourname/go-news-llm/backend/internal/graphql"
)

func main() {
	logger := log.Default()

	// Keep local SQLite ergonomics while supporting production Postgres via DATABASE_URL.
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		if err := os.MkdirAll("./data", 0o755); err != nil {
			logger.Fatalf("mkdir data: %v", err)
		}
	}
	d, err := db.OpenFromEnv()
	if err != nil {
		logger.Fatalf("db open: %v", err)
	}
	defer d.Close()

	var nc *nats.Conn
	if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
		bc, err := bus.Connect(natsURL)
		if err != nil {
			logger.Printf("nats disabled (connect failed): %v", err)
		} else {
			defer bc.Close()
			nc = bc.Conn
		}
	} else {
		logger.Printf("nats disabled (NATS_URL not set)")
	}

	hub := events.NewHub[db.NewsItem]()
	if nc != nil {
		if _, err := nc.Subscribe(workerSubjectNew(), func(m *nats.Msg) {
			var it db.NewsItem
			if err := json.Unmarshal(m.Data, &it); err != nil {
				return
			}
			hub.Publish(it)
		}); err != nil {
			logger.Fatalf("nats subscribe: %v", err)
		}
	}

	gql := graphqlsrv.New(graphqlsrv.Deps{DB: d, NC: nc})

	mux := http.NewServeMux()
	mux.Handle("/", gql.Playground())
	// Relay expects POST + JSON; a browser GET to /graphql would otherwise fail (often "EOF").
	mux.Handle("/graphql", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			gql.Playground().ServeHTTP(w, r)
			return
		}
		gql.Handler().ServeHTTP(w, r)
	}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		handleSSE(w, r, hub)
	})

	addr := env("API_ADDR", ":8080")
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           cors(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Printf("api listening on %s", addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}

func workerSubjectNew() string { return "news.new" }

func handleSSE(w http.ResponseWriter, r *http.Request, hub *events.Hub[db.NewsItem]) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	write := func(s string) error {
		_, err := w.Write([]byte(s))
		if err == nil {
			flusher.Flush()
		}
		return err
	}

	_ = write("event: ready\ndata: {}\n\n")

	ctx := r.Context()
	ch := hub.Subscribe(ctx, 64)
	for {
		select {
		case <-ctx.Done():
			return
		case it, ok := <-ch:
			if !ok {
				return
			}
			if err := events.SSEWriteJSON(write, "news", it); err != nil {
				return
			}
		}
	}
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", env("CORS_ORIGIN", "http://localhost:5173"))
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func env(k, def string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	return v
}

