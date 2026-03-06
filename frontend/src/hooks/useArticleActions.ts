import type { Dispatch, SetStateAction } from "react";
import type { ApiClient } from "@/api";
import type { Article } from "@/types";

export type TranslationParagraph = {
  index: number;
  source: string;
  translated: string;
  status: "pending" | "done";
};

type UseArticleActionsParams = {
  client: ApiClient;
  selectedArticle: Article | null;
  setSelectedArticle: Dispatch<SetStateAction<Article | null>>;
  setArticles: Dispatch<SetStateAction<Article[]>>;
  setMessage: (message: string, isError?: boolean) => void;
  isExtractingReadable: boolean;
  setIsExtractingReadable: Dispatch<SetStateAction<boolean>>;
  isRefreshingArticleCache: boolean;
  setIsRefreshingArticleCache: Dispatch<SetStateAction<boolean>>;
  isTranslatingArticle: boolean;
  setIsTranslatingArticle: Dispatch<SetStateAction<boolean>>;
  aiTargetLang: string;
  setTranslationParagraphsByArticleID: Dispatch<SetStateAction<Record<number, TranslationParagraph[]>>>;
};

export function useArticleActions({
  client,
  selectedArticle,
  setSelectedArticle,
  setArticles,
  setMessage,
  isExtractingReadable,
  setIsExtractingReadable,
  isRefreshingArticleCache,
  setIsRefreshingArticleCache,
  isTranslatingArticle,
  setIsTranslatingArticle,
  aiTargetLang,
  setTranslationParagraphsByArticleID,
}: UseArticleActionsParams) {
  const markUnread = async () => {
    if (!selectedArticle) {
      return;
    }
    try {
      const updated = await client.setArticleRead(selectedArticle.id, false);
      setSelectedArticle(updated);
      setArticles((current) => current.map((entry) => (entry.id === updated.id ? { ...entry, is_read: updated.is_read } : entry)));
      setMessage("文章已标记为未读");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const toggleFavorite = async () => {
    if (!selectedArticle) {
      return;
    }
    try {
      const updated = await client.setArticleFavorite(selectedArticle.id, !selectedArticle.is_favorite);
      setSelectedArticle(updated);
      setArticles((current) =>
        current.map((entry) =>
          entry.id === updated.id ? { ...entry, is_favorite: updated.is_favorite, favorited_at: updated.favorited_at } : entry,
        ),
      );
      setMessage(updated.is_favorite ? "已加入收藏" : "已取消收藏");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const extractReadableContent = async () => {
    if (!selectedArticle || isExtractingReadable) {
      return;
    }
    setIsExtractingReadable(true);
    setMessage("正在使用 Readability 抓取原文...");
    try {
      const updated = await client.extractArticleReadable(selectedArticle.id);
      setSelectedArticle(updated);
      setArticles((current) => current.map((entry) => (entry.id === updated.id ? { ...entry, full_content: updated.full_content } : entry)));
      setMessage("原文抓取完成");
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      setIsExtractingReadable(false);
    }
  };

  const refreshCurrentArticleCache = async () => {
    if (!selectedArticle || isRefreshingArticleCache) {
      return;
    }
    setIsRefreshingArticleCache(true);
    setMessage("正在刷新文章缓存...");
    try {
      const updated = await client.refreshArticleCache(selectedArticle.id);
      setSelectedArticle(updated);
      setArticles((current) => current.map((entry) => (entry.id === updated.id ? { ...entry, ...updated } : entry)));
      setTranslationParagraphsByArticleID((current) => ({ ...current, [updated.id]: [] }));
      setMessage("文章缓存已刷新");
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      setIsRefreshingArticleCache(false);
    }
  };

  const translateArticle = async () => {
    if (!selectedArticle || isTranslatingArticle) {
      return;
    }
    const articleID = selectedArticle.id;
    setIsTranslatingArticle(true);
    setMessage("正在调用 AI 翻译...");
    setTranslationParagraphsByArticleID((current) => ({ ...current, [articleID]: [] }));
    try {
      await client.translateArticleStream(articleID, aiTargetLang.trim() || "zh-CN", (event) => {
        if (event.type === "start") {
          const paragraphs: TranslationParagraph[] = Array.from({ length: Math.max(0, event.total) }, (_, idx) => ({
            index: idx + 1,
            source: event.sources?.[idx] || "",
            translated: "",
            status: "pending",
          }));
          setTranslationParagraphsByArticleID((current) => ({ ...current, [articleID]: paragraphs }));
          return;
        }
        if (event.type === "chunk") {
          setTranslationParagraphsByArticleID((current) => {
            const existing = current[articleID] || [];
            const next = [...existing];
            const targetIndex = Math.max(0, event.index - 1);
            while (next.length < event.total) {
              next.push({
                index: next.length + 1,
                source: "",
                translated: "",
                status: "pending",
              });
            }
            next[targetIndex] = {
              index: event.index,
              source: event.source || "",
              translated: event.translated || "",
              status: "done",
            };
            return { ...current, [articleID]: next };
          });
          return;
        }
        if (event.type === "error") {
          throw new Error(event.error || "translation stream failed");
        }
      });
      setMessage("AI 翻译完成");
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      setIsTranslatingArticle(false);
    }
  };

  return {
    markUnread,
    toggleFavorite,
    extractReadableContent,
    refreshCurrentArticleCache,
    translateArticle,
  };
}
