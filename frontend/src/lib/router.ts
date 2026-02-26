export const APP_ROUTES = {
  home: "/",
  article: "/articles/:id",
} as const;

export function buildArticleRoute(id: number): string {
  return `/articles/${id}`;
}

export function parseRouteArticleID(rawID: string | undefined): number | null {
  if (!rawID) return null;
  const parsed = Number(rawID);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return null;
  }
  return parsed;
}
