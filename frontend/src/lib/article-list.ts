import type { Article, RecommendationScores } from "@/types";

export type ReadFilter = "all" | "unread";
export type SortMode = "latest" | "oldest" | "recommend" | "quality" | "relevance" | "novelty";

export const SORT_MODE_LABELS: Record<SortMode, string> = {
  latest: "最新优先",
  oldest: "最早优先",
  recommend: "综合推荐",
  quality: "质量优先",
  relevance: "相关性优先",
  novelty: "新颖性优先",
};

function articleTimestamp(article: Article): number {
  const source = article.published_at || article.created_at || "";
  const ts = Date.parse(source);
  return Number.isNaN(ts) ? 0 : ts;
}

function clampScore(value: number): number {
  return Math.max(0, Math.min(100, Math.round(value)));
}

function normalizeByCap(value: number, cap: number): number {
  if (cap <= 0) {
    return 0;
  }
  return Math.min(value, cap) / cap;
}

function tokenize(text: string): string[] {
  return text.toLowerCase().match(/[\p{L}\p{N}]{2,}/gu) ?? [];
}

function computeHeuristicScores(article: Article): RecommendationScores {
  const title = (article.title || "").trim();
  const summary = (article.summary || "").trim();
  const fullContent = (article.full_content || "").trim();
  const body = fullContent || summary;

  const titleTokens = Array.from(new Set(tokenize(title)));
  const bodyTokenSet = new Set(tokenize(body));
  const overlapCount = titleTokens.filter((token) => bodyTokenSet.has(token)).length;
  const overlapRatio = titleTokens.length > 0 ? overlapCount / titleTokens.length : 0;

  const quality =
    12 +
    normalizeByCap(title.length, 80) * 20 +
    normalizeByCap(summary.length, 480) * 22 +
    normalizeByCap(fullContent.length, 4000) * 28 +
    (article.cover_url ? 6 : 0) +
    (article.link ? 6 : 0) +
    (article.published_at ? 6 : 0);

  const relevance =
    18 +
    overlapRatio * 52 +
    normalizeByCap(titleTokens.length, 10) * 12 +
    normalizeByCap(bodyTokenSet.size, 48) * 18;

  const ageHours = Math.max(0, (Date.now() - articleTimestamp(article)) / (1000 * 60 * 60));
  const freshness =
    ageHours <= 24 ? 42 : ageHours <= 72 ? 34 : ageHours <= 24 * 7 ? 24 : ageHours <= 24 * 30 ? 14 : 6;
  const novelty =
    freshness +
    normalizeByCap(title.length, 72) * 16 +
    normalizeByCap(fullContent.length || summary.length, 3000) * 18 +
    (article.cover_url ? 8 : 0) +
    (fullContent ? 8 : 0);

  const normalizedQuality = clampScore(quality);
  const normalizedRelevance = clampScore(relevance);
  const normalizedNovelty = clampScore(novelty);

  return {
    quality: normalizedQuality,
    relevance: normalizedRelevance,
    novelty: normalizedNovelty,
    composite: clampScore(normalizedQuality * 0.45 + normalizedRelevance * 0.35 + normalizedNovelty * 0.2),
  };
}

export function getRecommendationScores(article: Article): RecommendationScores {
  const current = article.recommendation_scores;
  if (current) {
    return {
      quality: clampScore(current.quality),
      relevance: clampScore(current.relevance),
      novelty: clampScore(current.novelty),
      composite: clampScore(current.composite),
    };
  }

  return computeHeuristicScores(article);
}

export function formatRecommendationSummary(article: Article): string {
  const scores = getRecommendationScores(article);
  return `综 ${scores.composite} / 质 ${scores.quality} / 相关 ${scores.relevance} / 新 ${scores.novelty}`;
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
  stickyUnreadIDs: Set<number>,
  preserveIncomingOrder = false,
): Article[] {
  const readFiltered = articles.filter((article) => {
    if (readFilter !== "unread") {
      return true;
    }
    if (!article.is_read) {
      return true;
    }
    return stickyUnreadIDs.has(article.id);
  });

  if (preserveIncomingOrder) {
    return readFiltered;
  }

  const withSignals = readFiltered.map((article) => ({
    article,
    timestamp: articleTimestamp(article),
    scores: getRecommendationScores(article),
  }));

  withSignals.sort((a, b) => {
    if (sortMode === "latest") {
      return b.timestamp - a.timestamp;
    }
    if (sortMode === "oldest") {
      return a.timestamp - b.timestamp;
    }

    const byScore =
      sortMode === "recommend"
        ? b.scores.composite - a.scores.composite
        : sortMode === "quality"
          ? b.scores.quality - a.scores.quality
          : sortMode === "relevance"
            ? b.scores.relevance - a.scores.relevance
            : b.scores.novelty - a.scores.novelty;

    if (byScore !== 0) {
      return byScore;
    }
    return b.timestamp - a.timestamp;
  });

  return withSignals.map((entry) => entry.article);
}
