import type { Article } from "@/types";

export function canRequestReadability(article: Article | null): boolean {
  return Boolean(article);
}

type AutoFetchParams = {
  article: Article | null;
  hasUsableFullContent: boolean;
  isExtractingReadable: boolean;
};

export function shouldAutoFetchReadability({ article, hasUsableFullContent, isExtractingReadable }: AutoFetchParams): boolean {
  if (!article) {
    return false;
  }
  if (hasUsableFullContent) {
    return false;
  }
  return !isExtractingReadable;
}
