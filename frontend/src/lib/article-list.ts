import type { Article } from "../types";

export type ReadFilter = "all" | "unread";
export type SortMode = "latest" | "oldest";

function articleTimestamp(article: Article): number {
  const source = article.published_at || article.created_at || "";
  const ts = Date.parse(source);
  return Number.isNaN(ts) ? 0 : ts;
}

export function formatArticleTime(raw: string | undefined): string {
  const text = (raw || "").trim();
  if (!text) {
    return "-";
  }

  const ts = Date.parse(text);
  if (Number.isNaN(ts)) {
    return "-";
  }

  const now = Date.now();
  const deltaMs = now - ts;
  if (deltaMs >= 0 && deltaMs < 60 * 1000) {
    return "刚刚";
  }
  if (deltaMs >= 0 && deltaMs < 24 * 60 * 60 * 1000) {
    const totalMinutes = Math.max(1, Math.floor(deltaMs / (60 * 1000)));
    const hours = Math.floor(totalMinutes / 60);
    const minutes = totalMinutes % 60;
    if (hours <= 0) {
      return `${minutes}分钟前`;
    }
    if (minutes === 0) {
      return `${hours}小时前`;
    }
    return `${hours}小时${minutes}分钟前`;
  }

  const utc8Ms = ts + 8 * 60 * 60 * 1000;
  const date = new Date(utc8Ms);
  const year = date.getUTCFullYear();
  const month = String(date.getUTCMonth() + 1).padStart(2, "0");
  const day = String(date.getUTCDate()).padStart(2, "0");
  return `${year}/${month}/${day}`;
}

export function filterAndSortArticles(
  articles: Article[],
  readFilter: ReadFilter,
  sortMode: SortMode,
  selectedArticleID: number | null,
): Article[] {
  const readFiltered = articles.filter((article) => {
    if (readFilter !== "unread") {
      return true;
    }
    if (!article.is_read) {
      return true;
    }
    return selectedArticleID === article.id;
  });

  const withTimestamp = readFiltered.map((article) => ({
    article,
    timestamp: articleTimestamp(article),
  }));

  withTimestamp.sort((a, b) => {
    if (sortMode === "latest") {
      return b.timestamp - a.timestamp;
    }
    return a.timestamp - b.timestamp;
  });

  return withTimestamp.map((entry) => entry.article);
}
