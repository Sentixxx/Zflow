import type { Article } from "@/types";

export function canRequestReadability(article: Article | null): boolean {
  return Boolean(article);
}
