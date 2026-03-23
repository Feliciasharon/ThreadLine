# ThreadLine - AI Powered Live News Feed


<img width="1490" height="727" alt="image" src="https://github.com/user-attachments/assets/9d48906c-169d-4ff4-aeba-f92b656984b4" />



- **Go backend**: GraphQL API + SSE “live updates”
- **Worker service**: fetches live news (RSS), summarizes via **Gemini (optional)** or **local Ollama (optional)**, stores in DB
- **Distributed component**: **NATS pub/sub** (local + Docker)
- **Beautiful frontend** (coming next): Vite + React + Tailwind

## News sources

Currently pulled via RSS in `backend/internal/news/fetch.go`:

- BBC World
- The Guardian World
- NYT World
- NPR World

## Local run (recommended)

Prereqs:

- Docker Desktop
- Go 1.23+ (if you want to run without Docker)
- Optional: Ollama (local LLM) if you don’t want any API keys

Start everything:

```bash
docker compose up
```

**Gemini (optional, Docker):** create a `.env` in the repo root (gitignored) with `GEMINI_API_KEY=...` and optionally `GEMINI_MODEL=gemini-2.0-flash`. Compose passes these into the worker automatically.

Then open:

- GraphQL playground: `http://localhost:8080/` or `http://localhost:8080/graphql` (same UI; use **POST** to `/graphql` for API clients only)
- GraphQL endpoint (POST): `http://localhost:8080/graphql`
- Live events (SSE): `http://localhost:8080/events`

Trigger a refresh:

```graphql
mutation { refresh }
```

Query news:

```graphql
query { news(limit: 20) { title source url publishedAt summary } }
```

## Summarization modes

- **Gemini (free tier until quota is hit)**:
  - Set `GEMINI_API_KEY` (default model is `gemini-2.0-flash`; override with `GEMINI_MODEL` if Google renames models — use **ListModels** in AI Studio if you get 404)
  - RSS HTML is stripped before summarizing; restart the worker or trigger a refresh so existing DB rows get new summaries.
  - When quota is exceeded, you can remove the key and it will fall back automatically.
- **Ollama (fully free/local)**:
  - Run Ollama locally, e.g. `ollama serve`
  - Set `OLLAMA_BASE_URL=http://host.docker.internal:11434`
- **Fallback (no LLM)**:
  - Set nothing; you’ll still get short summaries.

## Storage modes

- **Local default**: SQLite via `DB_PATH=./data/news.db`
- **Production default**: Postgres via `DATABASE_URL` (used automatically when set)

This makes Render robust because API + worker share the same durable database.

## Deploy 

Target:

- **Vercel**: frontend
- **Render**: Go API + worker


### Vercel (frontend)

- Import the `frontend/` folder as a Vercel project
- Set env var:
  - `VITE_API_URL` = your Render API base URL (example: `https://go-news-llm-api.onrender.com`)

### Render (backend)

- Create a new Render “Blueprint” from `render.yaml`
- The blueprint provisions a shared free Postgres database (`go-news-llm-db`)
- Both API and worker receive `DATABASE_URL` from that same DB
- After Vercel deploy, set `CORS_ORIGIN` on the Render API to your Vercel domain
- Optional: set `GEMINI_API_KEY` on the worker


### Steps to run after deploying:
1. Turn on the Render PostgreSQL db and webservice (ThreadLine)
2. run ```docker compose down && docker compose up --build ``` locally as you need to subscribe to deploy it on Render (background worker)
3. Access Vercel - https://thread-line-718xl3acp-feliciasharons-projects.vercel.app/

   
