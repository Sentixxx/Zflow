import type { Feed } from "../types";

export function feedHost(rawURL: string | undefined): string {
  const text = (rawURL || "").trim();
  if (!text) return "";
  try {
    return new URL(text).host.toLowerCase();
  } catch {
    return "";
  }
}

export function buildFeedIconURLByHost(feeds: Feed[], apiBase: string): Map<string, string> {
  const map = new Map<string, string>();
  const base = apiBase.replace(/\/$/, "");
  for (const feed of feeds) {
    const host = feedHost(feed.url);
    if (!host || !feed.icon_url || map.has(host)) {
      continue;
    }
    map.set(host, `${base}${feed.icon_url}`);
  }
  return map;
}
