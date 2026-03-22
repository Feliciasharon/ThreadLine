package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type DB struct {
	SQL    *sql.DB
	driver string
}

func OpenFromEnv() (*DB, error) {
	// Production-friendly default: if DATABASE_URL is present, use Postgres.
	if dsn := strings.TrimSpace(os.Getenv("DATABASE_URL")); dsn != "" {
		return OpenPostgres(dsn)
	}
	path := strings.TrimSpace(os.Getenv("DB_PATH"))
	if path == "" {
		path = "./data/news.db"
	}
	return OpenSQLite(path)
}

func OpenSQLite(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Make SQLite behave a bit better under concurrent reads/writes.
	if _, err := sqlDB.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	d := &DB{SQL: sqlDB, driver: "sqlite"}
	if err := d.init(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return d, nil
}

func OpenPostgres(dsn string) (*DB, error) {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	d := &DB{SQL: sqlDB, driver: "pgx"}
	if err := d.init(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) Close() error { return d.SQL.Close() }

func (d *DB) init() error {
	schema := `
CREATE TABLE IF NOT EXISTS news_items (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  url TEXT NOT NULL UNIQUE,
  source TEXT NOT NULL,
  published_at TIMESTAMPTZ NOT NULL,
  summary TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_news_published_at ON news_items(published_at);
`
	_, err := d.SQL.Exec(schema)
	return err
}

type NewsItem struct {
	ID          string
	Title       string
	URL         string
	Source      string
	PublishedAt time.Time
	Summary     string
}

func (d *DB) UpsertNews(ctx context.Context, items []NewsItem) (int, error) {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, d.bind(`
INSERT INTO news_items (id, title, url, source, published_at, summary)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(url) DO UPDATE SET
  title=excluded.title,
  source=excluded.source,
  published_at=excluded.published_at,
  summary=excluded.summary
`))
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	n := 0
	for _, it := range items {
		if it.PublishedAt.IsZero() {
			it.PublishedAt = time.Now().UTC()
		}
		_, err := stmt.ExecContext(ctx,
			it.ID,
			it.Title,
			it.URL,
			it.Source,
			it.PublishedAt.UTC(),
			it.Summary,
		)
		if err != nil {
			return 0, err
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

func (d *DB) ListNews(ctx context.Context, limit int, since *time.Time) ([]NewsItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var (
		rows *sql.Rows
		err  error
	)
	if since != nil {
		rows, err = d.SQL.QueryContext(ctx, d.bind(`
SELECT id, title, url, source, published_at, summary
FROM news_items
WHERE published_at >= ?
ORDER BY published_at DESC
LIMIT ?
`), since.UTC(), limit)
	} else {
		rows, err = d.SQL.QueryContext(ctx, d.bind(`
SELECT id, title, url, source, published_at, summary
FROM news_items
ORDER BY published_at DESC
LIMIT ?
`), limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]NewsItem, 0, limit)
	for rows.Next() {
		var (
			it  NewsItem
			pub any
		)
		if err := rows.Scan(&it.ID, &it.Title, &it.URL, &it.Source, &pub, &it.Summary); err != nil {
			return nil, err
		}
		switch v := pub.(type) {
		case time.Time:
			it.PublishedAt = v.UTC()
		case string:
			t, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				it.PublishedAt = time.Now().UTC()
			} else {
				it.PublishedAt = t.UTC()
			}
		case []byte:
			t, err := time.Parse(time.RFC3339Nano, string(v))
			if err != nil {
				it.PublishedAt = time.Now().UTC()
			} else {
				it.PublishedAt = t.UTC()
			}
		default:
			it.PublishedAt = time.Now().UTC()
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) bind(q string) string {
	if d.driver != "pgx" {
		return q
	}
	var (
		b strings.Builder
		n int
	)
	b.Grow(len(q) + 16)
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
			b.WriteString(fmt.Sprintf("$%d", n))
		} else {
			b.WriteByte(q[i])
		}
	}
	return b.String()
}

