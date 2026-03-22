package graphqlsrv

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	graphql "github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/nats-io/nats.go"

	"github.com/yourname/go-news-llm/backend/internal/db"
)

type Server struct {
	Schema *graphql.Schema
}

type Deps struct {
	DB *db.DB
	NC *nats.Conn
}

func New(deps Deps) *Server {
	schema := graphql.MustParseSchema(schemaSDL, &rootResolver{deps: deps})
	return &Server{Schema: schema}
}

func (s *Server) Handler() http.Handler {
	return &relay.Handler{Schema: s.Schema}
}

func (s *Server) Playground() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(playgroundHTML))
	})
}

type rootResolver struct {
	deps Deps
}

type newsArgs struct {
	// GraphQL Int arguments are unmarshaled as int32 in graph-gophers/graphql-go.
	// Do not use `limit: Int = 50` in the schema with this type — defaults break unmarshaling.
	Limit *int32
	Since *graphql.Time
}

func (r *rootResolver) News(ctx context.Context, args newsArgs) ([]*newsItemResolver, error) {
	limit := 50
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	var since *time.Time
	if args.Since != nil {
		t := args.Since.Time
		since = &t
	}
	items, err := r.deps.DB.ListNews(ctx, limit, since)
	if err != nil {
		return nil, err
	}
	out := make([]*newsItemResolver, 0, len(items))
	for _, it := range items {
		it := it
		out = append(out, &newsItemResolver{it: it})
	}
	return out, nil
}

func (r *rootResolver) Refresh(ctx context.Context) (bool, error) {
	// Best-effort request; worker also refreshes on a schedule.
	if r.deps.NC != nil {
		msg, err := r.deps.NC.Request("news.refresh", []byte("{}"), 2*time.Second)
		if err == nil && msg != nil {
			_ = json.Unmarshal(msg.Data, &struct{}{})
		}
	}
	return true, nil
}

type newsItemResolver struct {
	it db.NewsItem
}

func (n *newsItemResolver) ID() graphql.ID     { return graphql.ID(n.it.ID) }
func (n *newsItemResolver) Title() string     { return n.it.Title }
func (n *newsItemResolver) URL() string       { return n.it.URL }
func (n *newsItemResolver) Source() string    { return n.it.Source }
func (n *newsItemResolver) PublishedAt() graphql.Time {
	return graphql.Time{Time: n.it.PublishedAt}
}
func (n *newsItemResolver) Summary() string { return n.it.Summary }

const schemaSDL = `
schema {
  query: Query
  mutation: Mutation
}

scalar Time

type NewsItem {
  id: ID!
  title: String!
  url: String!
  source: String!
  publishedAt: Time!
  summary: String!
}

type Query {
  news(limit: Int, since: Time): [NewsItem!]!
}

type Mutation {
  refresh: Boolean!
}
`

// Self-contained playground: no Apollo Sandbox iframe (often blocked by network / refuses to connect).
const playgroundHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>GraphQL — local</title>
  <style>
    :root { --bg: #0f1419; --panel: #1a2332; --text: #e7ecf3; --muted: #8b98a8; --accent: #5b8def; }
    * { box-sizing: border-box; }
    body { margin: 0; font-family: ui-sans-serif, system-ui, sans-serif; background: var(--bg); color: var(--text); min-height: 100vh; }
    header { padding: 12px 20px; border-bottom: 1px solid #2a3544; background: var(--panel); }
    header h1 { margin: 0; font-size: 1rem; font-weight: 600; }
    header p { margin: 4px 0 0; font-size: 12px; color: var(--muted); }
    main { display: grid; grid-template-columns: 1fr 1fr; gap: 0; min-height: calc(100vh - 72px); }
    @media (max-width: 900px) { main { grid-template-columns: 1fr; } }
    label { display: block; font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--muted); margin: 12px 16px 6px; }
    textarea, pre { width: 100%; margin: 0; padding: 16px; font-family: ui-monospace, monospace; font-size: 13px; line-height: 1.5; border: none; resize: vertical; }
    textarea { min-height: 220px; background: #0b0f14; color: #c8d4e0; border-right: 1px solid #2a3544; }
    pre { min-height: 220px; background: #0b0f14; color: #a8d4a8; overflow: auto; margin: 0; white-space: pre-wrap; word-break: break-word; }
    .col { display: flex; flex-direction: column; min-height: 0; }
    .toolbar { padding: 10px 16px; background: var(--panel); border-bottom: 1px solid #2a3544; display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
    button { background: var(--accent); color: #fff; border: none; padding: 8px 16px; border-radius: 8px; font-weight: 600; cursor: pointer; font-size: 13px; }
    button:hover { filter: brightness(1.08); }
    .err { color: #f87171; }
  </style>
</head>
<body>
  <header>
    <h1>GraphQL playground (local)</h1>
    <p>Runs queries against <strong>this host</strong> — no external UI (Apollo Sandbox not required).</p>
  </header>
  <main>
    <div class="col">
      <div class="toolbar">
        <button type="button" id="run">Run query</button>
        <span style="font-size:12px;color:var(--muted)">POST → <code id="ep"></code></span>
      </div>
      <label for="q">Query / mutation</label>
      <textarea id="q" spellcheck="false">query {
  news(limit: 5) {
    title
    source
    summary
    url
  }
}</textarea>
    </div>
    <div class="col">
      <label for="out">Response</label>
      <pre id="out">Click “Run query”.</pre>
    </div>
  </main>
  <script>
    const ep = window.location.origin + "/graphql";
    document.getElementById("ep").textContent = ep;
    document.getElementById("run").onclick = async () => {
      const out = document.getElementById("out");
      out.textContent = "Loading…";
      out.className = "";
      try {
        const r = await fetch(ep, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ query: document.getElementById("q").value })
        });
        const text = await r.text();
        try { out.textContent = JSON.stringify(JSON.parse(text), null, 2); }
        catch { out.textContent = text || "(empty body)"; }
        if (!r.ok) out.className = "err";
      } catch (e) {
        out.className = "err";
        out.textContent = String(e);
      }
    };
  </script>
</body>
</html>`

