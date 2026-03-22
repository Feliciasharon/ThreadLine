import { useEffect, useMemo, useState } from "react";
import clsx from "clsx";
import { eventsURL, fetchNews, refresh, type NewsItem } from "../lib/api";
import { displaySummary, displayTitle, stripHtml } from "../lib/text";

function fmtTime(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

export function App() {
  const [items, setItems] = useState<NewsItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [live, setLive] = useState(true);
  const [query, setQuery] = useState("");

  const validArticles = useMemo(() => {
    return (items || []).filter(
      (a) =>
        a.title &&
        a.title.trim() !== "" &&
        a.summary &&
        a.summary.trim() !== ""
    );
  }, [items]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return validArticles;
    return validArticles.filter((it) =>
      (stripHtml(it.title) + " " + stripHtml(it.summary) + " " + it.source).toLowerCase().includes(q)
    );
  }, [validArticles, query]);

  async function load(opts?: { silent?: boolean }) {
    const silent = opts?.silent === true;
    if (!silent) {
      setLoading(true);
      setError(null);
    }
    try {
      const news = await fetchNews(60);
      setItems(news);
    } catch (e: any) {
      if (!silent) {
        setError(e?.message ?? "Failed to load news");
      }
      // Never clear the list on background / retry failures.
    } finally {
      if (!silent) {
        setLoading(false);
      }
    }
  }

  useEffect(() => {
    void load();
  }, []);

  // Live updates:
  // - If NATS is enabled, backend pushes SSE "news" events.
  // - On SSE errors (CORS, proxy, idle), fall back to quiet polling — without wiping the UI.
  useEffect(() => {
    if (!live) return;

    let cancelled = false;
    let pollId: ReturnType<typeof setInterval> | null = null;
    const es = new EventSource(eventsURL());

    const startPolling = () => {
      if (pollId != null || cancelled) return;
      pollId = window.setInterval(() => {
        void load({ silent: true });
      }, 15_000);
    };

    es.addEventListener("news", (evt) => {
      try {
        const it = JSON.parse((evt as MessageEvent).data) as NewsItem;
        setItems((prev) => {
          const exists = prev.some((p) => p.id === it.id);
          if (exists) return prev;
          return [it, ...prev].slice(0, 80);
        });
      } catch {
        // ignore
      }
    });

    es.onerror = () => {
      es.close();
      startPolling();
    };

    return () => {
      cancelled = true;
      es.close();
      if (pollId != null) {
        window.clearInterval(pollId);
        pollId = null;
      }
    };
  }, [live]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="min-h-screen">
      <div className="mx-auto max-w-6xl px-5 py-10">
        <header className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div>
            <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3 py-1 text-xs text-white/70">
              <span className="h-2 w-2 rounded-full bg-emerald-400" />
              Live news + AI Powered Summaries
            </div>
            <h1 className="mt-3 text-3xl font-semibold tracking-tight">Threadline - AI Powered Live News Feed</h1>
          </div>

          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
            <div className="relative">
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search headlines, summaries, sources…"
                className="w-full rounded-xl border border-white/10 bg-white/5 px-4 py-2 text-sm text-white placeholder:text-white/40 outline-none ring-0 focus:border-indigo-400/50"
              />
            </div>

            <button
              onClick={() => setLive((v) => !v)}
              className={clsx(
                "rounded-xl border px-4 py-2 text-sm font-medium transition",
                live
                  ? "border-emerald-400/30 bg-emerald-400/10 text-emerald-200 hover:bg-emerald-400/15"
                  : "border-white/10 bg-white/5 text-white/80 hover:bg-white/10"
              )}
            >
              {live ? "Live: on" : "Live: off"}
            </button>

            <button
              onClick={async () => {
                await refresh().catch(() => {});
                await load();
              }}
              className="rounded-xl border border-indigo-400/30 bg-indigo-400/10 px-4 py-2 text-sm font-medium text-indigo-200 hover:bg-indigo-400/15"
            >
              Refresh
            </button>
          </div>
        </header>

        <main className="mt-8">
          <div className="rounded-2xl border border-white/10 bg-white/5 p-5">
            <div className="flex items-center justify-between">
              <div className="text-sm text-white/70">
                {loading ? "Loading…" : `${filtered.length} stories`}
                {error ? <span className="ml-3 text-rose-300">{error}</span> : null}
              </div>
              <div className="text-xs text-white/50">GraphQL + SSE</div>
            </div>

            <div className="mt-5 grid gap-4">
              {filtered.map((it) => (
                <article
                  key={it.id}
                  className="group rounded-2xl border border-white/10 bg-black/20 p-5 transition hover:border-white/20 hover:bg-black/30"
                >
                  <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
                    <div className="min-w-0">
                      <a
                        href={it.url}
                        target="_blank"
                        rel="noreferrer"
                        className="block truncate text-base font-semibold text-white group-hover:text-white"
                        title={displayTitle(it.title, it.source, it.url)}
                      >
                        {displayTitle(it.title, it.source, it.url)}
                      </a>
                      <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-white/60">
                        <span className="rounded-full border border-white/10 bg-white/5 px-2 py-0.5">{it.source}</span>
                        <span className="text-white/40">•</span>
                        <span>{fmtTime(it.publishedAt)}</span>
                      </div>
                    </div>
                  </div>

                  <p className="mt-3 text-sm leading-relaxed text-white/75">
                    {displaySummary(it.summary, it.title)}
                  </p>

                  <div className="mt-4">
                    <a
                      href={it.url}
                      target="_blank"
                      rel="noreferrer"
                      className="text-sm font-medium text-indigo-200 hover:text-indigo-100"
                    >
                      Read full story →
                    </a>
                  </div>
                </article>
              ))}

              {!loading && filtered.length === 0 ? (
                <div className="rounded-2xl border border-white/10 bg-black/20 p-6 text-sm text-white/70">
                  No matches. Try a different search term.
                </div>
              ) : null}
            </div>
          </div>
        </main>

        
      </div>
    </div>
  );
}

