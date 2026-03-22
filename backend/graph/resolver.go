package graph

import (
	"github.com/nats-io/nats.go"

	"github.com/yourname/go-news-llm/backend/internal/db"
	"github.com/yourname/go-news-llm/backend/internal/events"
)

type Resolver struct {
	DB  *db.DB
	NC  *nats.Conn
	Hub *events.Hub[db.NewsItem]
}

