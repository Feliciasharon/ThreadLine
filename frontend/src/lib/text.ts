/** Remove HTML tags for safe plain-text display (legacy DB rows, RSS). */
export function stripHtml(s: string): string {
  if (!s) return "";
  return s
    .replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, " ")
    .replace(/<style\b[^<]*(?:(?!<\/style>)<[^<]*)*<\/style>/gi, " ")
    .replace(/<!--[\s\S]*?-->/g, " ")
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

/** Prefer a real summary; fall back to title if summary is empty or junk. */
export function displaySummary(summary: string, title: string): string {
  const s = stripHtml(summary).trim();
  if (s.length > 0) return s;
  const t = stripHtml(title).trim();
  return t || "Summary not available.";
}

export function displayTitle(title: string, source: string, url: string): string {
  const t = stripHtml(title).trim();
  if (t) return t;
  const s = stripHtml(source).trim();
  if (s) return s;
  try {
    const u = new URL(url);
    return u.hostname || url || "Untitled";
  } catch {
    return url || "Untitled";
  }
}
