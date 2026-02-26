import { useEffect, useMemo, useRef, useState } from "react";
import { ApiClient } from "@/api";
import type { Article, Feed, Folder } from "@/types";
import { filterAndSortArticles } from "@/lib/article-list";
import type { ReadFilter, SortMode } from "@/lib/article-list";
import { sanitizeRichHTML } from "@/lib/sanitize";
import { buildFeedIconURLByHost, feedHost } from "@/lib/feed-utils";
import {
  RssFallbackIcon,
  TopBar,
  RefreshFailureBanner,
  ArticleListToolbar,
  ArticleList,
  ArticleDetailContent,
} from "@/components";
import type { RefreshFailure } from "@/components";
import { useReaderStore } from "@/stores/useReaderStore";
import { useArticleRoute } from "@/hooks/useArticleRoute";
import { useReaderQueries } from "@/hooks/useReaderQueries";
import { refreshFeedsBatch } from "@/services/feed-refresh-service";
import { useFeeds } from "@/hooks/useFeeds";
import { useEntries } from "@/hooks/useEntries";

const DEFAULT_API_BASE = "http://localhost:8080";
const PREFETCH_BATCH_SIZE = 20;
const VISIBLE_STEP_SIZE = 10;
const MIN_SIDEBAR_WIDTH = 260;
const MAX_SIDEBAR_WIDTH = 520;
const MIN_LIST_WIDTH = 300;
const MAX_LIST_WIDTH = 620;
const COLLAPSED_SIDEBAR_WIDTH = 56;
const RESIZER_WIDTH = 8;
const LOAD_MORE_COOLDOWN_MS = 80;
type ResizeTarget = "sidebar" | "list";
type FolderContextMenu = { folder: Folder; x: number; y: number } | null;
type FeedContextMenu = { feed: Feed; x: number; y: number } | null;
type ScriptLang = "shell" | "python" | "javascript";
type SettingsTab = "connection" | "subscription" | "script" | "ai" | "data";
type SidebarMode = "subscriptions" | "favorites";
type TranslationParagraph = {
  index: number;
  source: string;
  translated: string;
  status: "pending" | "done";
};

function normalizeScriptLang(raw: string | undefined): ScriptLang {
  if (raw === "python" || raw === "javascript") {
    return raw;
  }
  return "shell";
}

function scriptLangByFileName(name: string): ScriptLang | null {
  const lower = name.toLowerCase();
  if (lower.endsWith(".py")) return "python";
  if (lower.endsWith(".js") || lower.endsWith(".mjs") || lower.endsWith(".cjs")) return "javascript";
  if (lower.endsWith(".sh") || lower.endsWith(".bash") || lower.endsWith(".zsh")) return "shell";
  return null;
}

function toValidURL(raw: string | undefined): string {
  if (!raw) return "";
  try {
    return new URL(raw).toString();
  } catch {
    return "";
  }
}

export function App() {
  const [apiBase, setApiBase] = useState<string>(localStorage.getItem("zflow_api_base") || DEFAULT_API_BASE);
  const [networkProxyURL, setNetworkProxyURL] = useState<string>("");
  const [aiAPIKey, setAIAPIKey] = useState<string>("");
  const [aiBaseURL, setAIBaseURL] = useState<string>("");
  const [aiModel, setAIModel] = useState<string>("");
  const [aiTargetLang, setAITargetLang] = useState<string>("zh-CN");
  const [articleRetentionDays, setArticleRetentionDays] = useState<string>("90");
  const [feedURL, setFeedURL] = useState("");
  const [selectedArticle, setSelectedArticle] = useState<Article | null>(null);
  const selectedFeedID = useReaderStore((state) => state.selectedFeedID);
  const setSelectedFeedID = useReaderStore((state) => state.setSelectedFeedID);
  const selectedFolderID = useReaderStore((state) => state.selectedFolderID);
  const setSelectedFolderID = useReaderStore((state) => state.setSelectedFolderID);
  const [newFeedFolderID, setNewFeedFolderID] = useState<number | null>(null);
  const [scriptFeedID, setScriptFeedID] = useState<number | null>(null);
  const [scriptContent, setScriptContent] = useState<string>("");
  const [scriptLang, setScriptLang] = useState<ScriptLang>("shell");
  const [scriptDirty, setScriptDirty] = useState<boolean>(false);
  const readFilter = useReaderStore((state) => state.readFilter);
  const setReadFilter = useReaderStore((state) => state.setReadFilter);
  const sortMode = useReaderStore((state) => state.sortMode);
  const setSortMode = useReaderStore((state) => state.setSortMode);
  const [bufferedCount, setBufferedCount] = useState<number>(PREFETCH_BATCH_SIZE);
  const [visibleCount, setVisibleCount] = useState<number>(VISIBLE_STEP_SIZE);
  const [stickyUnreadIDs, setStickyUnreadIDs] = useState<number[]>([]);
  const [sidebarCollapsed, setSidebarCollapsed] = useState<boolean>(false);
  const [settingsOpen, setSettingsOpen] = useState<boolean>(false);
  const [sidebarWidth, setSidebarWidth] = useState<number>(360);
  const [listWidth, setListWidth] = useState<number>(360);
  const [isNarrow, setIsNarrow] = useState<boolean>(() => window.innerWidth <= 900);
  const [folderContextMenu, setFolderContextMenu] = useState<FolderContextMenu>(null);
  const [feedContextMenu, setFeedContextMenu] = useState<FeedContextMenu>(null);
  const [renamingFeedID, setRenamingFeedID] = useState<number | null>(null);
  const [renamingFeedTitle, setRenamingFeedTitle] = useState<string>("");
  const [renamingFeedOriginalTitle, setRenamingFeedOriginalTitle] = useState<string>("");
  const [manageCategoryFeed, setManageCategoryFeed] = useState<Feed | null>(null);
  const [manageCategoryFolderID, setManageCategoryFolderID] = useState<number | null>(null);
  const [settingsTab, setSettingsTab] = useState<SettingsTab>("subscription");
  const [sidebarMode, setSidebarMode] = useState<SidebarMode>("subscriptions");
  const [collapsedFolders, setCollapsedFolders] = useState<Record<number, boolean>>({});
  const [draggingFeedID, setDraggingFeedID] = useState<number | null>(null);
  const [dragOverFolderID, setDragOverFolderID] = useState<number | null>(null);
  const [dragOverUncategorized, setDragOverUncategorized] = useState<boolean>(false);
  const [dragOverDeleteZone, setDragOverDeleteZone] = useState<boolean>(false);
  const [pendingDeleteFeed, setPendingDeleteFeed] = useState<Feed | null>(null);
  const [listBounce, setListBounce] = useState<boolean>(false);
  const [isRefreshingArticles, setIsRefreshingArticles] = useState<boolean>(false);
  const [isRefreshingFeeds, setIsRefreshingFeeds] = useState<boolean>(false);
  const [isExtractingReadable, setIsExtractingReadable] = useState<boolean>(false);
  const [isRefreshingArticleCache, setIsRefreshingArticleCache] = useState<boolean>(false);
  const [isTranslatingArticle, setIsTranslatingArticle] = useState<boolean>(false);
  const [translationParagraphsByArticleID, setTranslationParagraphsByArticleID] = useState<Record<number, TranslationParagraph[]>>({});
  const [refreshFailures, setRefreshFailures] = useState<RefreshFailure[]>([]);
  const [status, setStatus] = useState("准备就绪");
  const [error, setError] = useState("");
  const lastLoadAtRef = useRef<number>(0);
  const bounceTimerRef = useRef<number | null>(null);
  const client = useMemo(() => new ApiClient(apiBase), [apiBase]);
  const { feedsQuery, foldersQuery, articlesInfiniteQuery } = useReaderQueries(apiBase);
  const sanitizedSummaryHTML = useMemo(() => sanitizeRichHTML(selectedArticle?.summary), [selectedArticle?.summary]);
  const sanitizedFullContentHTML = useMemo(() => sanitizeRichHTML(selectedArticle?.full_content), [selectedArticle?.full_content]);

  const setMessage = (message: string, isError = false) => {
    if (isError) {
      setError(message);
      setStatus("");
      return;
    }
    setError("");
    setStatus(message);
  };

  const { feeds, folders, loadFeeds, loadFolders } = useFeeds(client, feedsQuery, foldersQuery, setMessage);
  const { articles, setArticles, loadArticles, fetchNextArticlePage, hasNextArticlePage } = useEntries(client, articlesInfiniteQuery, setMessage);

  const folderNameByID = useMemo(() => {
    const map = new Map<number, string>();
    folders.forEach((folder) => map.set(folder.id, folder.name));
    return map;
  }, [folders]);
  const feedNameByID = useMemo(() => {
    const map = new Map<number, string>();
    feeds.forEach((feed) => map.set(feed.id, feed.title || feed.url || `#${feed.id}`));
    return map;
  }, [feeds]);
  const feedByID = useMemo(() => {
    const map = new Map<number, Feed>();
    feeds.forEach((feed) => map.set(feed.id, feed));
    return map;
  }, [feeds]);
  const feedIconURLByHost = useMemo(() => buildFeedIconURLByHost(feeds, apiBase), [feeds, apiBase]);

  const rootFolders = useMemo(() => folders.filter((folder) => folder.parent_id == null), [folders]);
  const childFoldersByParent = useMemo(() => {
    const map = new Map<number, Folder[]>();
    folders.forEach((folder) => {
      if (folder.parent_id == null) {
        return;
      }
      const list = map.get(folder.parent_id) || [];
      list.push(folder);
      map.set(folder.parent_id, list);
    });
    return map;
  }, [folders]);
  const feedsByFolder = useMemo(() => {
    const map = new Map<number, Feed[]>();
    feeds.forEach((feed) => {
      if (feed.folder_id == null) {
        return;
      }
      const list = map.get(feed.folder_id) || [];
      list.push(feed);
      map.set(feed.folder_id, list);
    });
    return map;
  }, [feeds]);
  const uncategorizedFeeds = useMemo(() => feeds.filter((feed) => feed.folder_id == null), [feeds]);
  const collectDescendantFolderIDs = (rootID: number): Set<number> => {
    const visited = new Set<number>();
    const stack: number[] = [rootID];
    while (stack.length > 0) {
      const current = stack.pop();
      if (current == null || visited.has(current)) {
        continue;
      }
      visited.add(current);
      const children = childFoldersByParent.get(current) || [];
      children.forEach((child) => stack.push(child.id));
    }
    return visited;
  };

  const filterArticlesByScope = (items: Article[], feedID: number | null, folderID: number | null): Article[] => {
    if (sidebarMode === "favorites") {
      return items.filter((article) => article.is_favorite);
    }
    if (feedID != null) {
      return items.filter((article) => article.feed_id === feedID);
    }
    if (folderID != null) {
      const folderIDs = collectDescendantFolderIDs(folderID);
      const feedIDs = new Set(feeds.filter((feed) => feed.folder_id != null && folderIDs.has(feed.folder_id)).map((feed) => feed.id));
      return items.filter((article) => feedIDs.has(article.feed_id));
    }
    return items;
  };
  const favoriteArticles = useMemo(
    () => [...articles.filter((article) => article.is_favorite)].sort((a, b) => (Date.parse(b.published_at || b.created_at) || 0) - (Date.parse(a.published_at || a.created_at) || 0)),
    [articles],
  );
  const rebuildStickyUnreadIDs = (items: Article[], feedID: number | null, folderID: number | null, nextReadFilter: ReadFilter) => {
    if (nextReadFilter !== "unread") {
      setStickyUnreadIDs([]);
      return;
    }
    const ids = filterArticlesByScope(items, feedID, folderID)
      .filter((article) => !article.is_read)
      .map((article) => article.id);
    setStickyUnreadIDs(ids);
  };
  const filteredAndSortedArticles = useMemo(() => {
    const filteredBySource = filterArticlesByScope(articles, selectedFeedID, selectedFolderID);
    return filterAndSortArticles(filteredBySource, readFilter, sortMode, new Set(stickyUnreadIDs));
  }, [articles, readFilter, sortMode, selectedFeedID, selectedFolderID, feeds, childFoldersByParent, stickyUnreadIDs, sidebarMode]);
  const effectiveBufferedCount = Math.min(bufferedCount, filteredAndSortedArticles.length);
  const pagedArticles = useMemo(
    () => filteredAndSortedArticles.slice(0, Math.min(visibleCount, effectiveBufferedCount)),
    [filteredAndSortedArticles, visibleCount, effectiveBufferedCount],
  );
  const articleListTitle = useMemo(() => {
    if (sidebarMode === "favorites") {
      return "收藏文章";
    }
    if (selectedFeedID != null) {
      return `订阅文章（${feedNameByID.get(selectedFeedID) || `#${selectedFeedID}`}）`;
    }
    if (selectedFolderID != null) {
      return `分类文章（${folderNameByID.get(selectedFolderID) || `#${selectedFolderID}`}）`;
    }
    return "全部文章";
  }, [sidebarMode, selectedFeedID, selectedFolderID, feedNameByID, folderNameByID]);
  const selectedArticleOpenURL = useMemo(() => {
    if (!selectedArticle) {
      return "";
    }
    const byArticleLink = toValidURL(selectedArticle.link);
    if (byArticleLink) {
      return byArticleLink;
    }
    const sourceFeed = feedByID.get(selectedArticle.feed_id);
    const byFeedURL = toValidURL(sourceFeed?.url);
    if (!byFeedURL) {
      return "";
    }
    return new URL(byFeedURL).origin;
  }, [selectedArticle, feedByID]);
  const currentTranslationParagraphs = useMemo(() => {
    if (!selectedArticle) {
      return [];
    }
    return translationParagraphsByArticleID[selectedArticle.id] || [];
  }, [selectedArticle, translationParagraphsByArticleID]);

  const handleSaveAPIBase = () => {
    localStorage.setItem("zflow_api_base", apiBase);
    setMessage("API Base 已保存");
  };

  const loadNetworkSettings = async () => {
    try {
      const data = await client.getNetworkSettings();
      setNetworkProxyURL((data.proxy_url || "").trim());
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const saveNetworkSettings = async () => {
    try {
      const data = await client.updateNetworkSettings(networkProxyURL.trim());
      setNetworkProxyURL((data.proxy_url || "").trim());
      setMessage("网络代理设置已保存并应用");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const loadAISettings = async () => {
    try {
      const data = await client.getAISettings();
      setAIAPIKey((data.api_key || "").trim());
      setAIBaseURL((data.base_url || "").trim());
      setAIModel((data.model || "").trim());
      setAITargetLang((data.target_lang || "zh-CN").trim() || "zh-CN");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const saveAISettings = async () => {
    try {
      const data = await client.updateAISettings({
        api_key: aiAPIKey.trim(),
        base_url: aiBaseURL.trim(),
        model: aiModel.trim(),
        target_lang: aiTargetLang.trim() || "zh-CN",
      });
      setAIAPIKey((data.api_key || "").trim());
      setAIBaseURL((data.base_url || "").trim());
      setAIModel((data.model || "").trim());
      setAITargetLang((data.target_lang || "zh-CN").trim() || "zh-CN");
      setMessage("AI 设置已保存");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const loadDataSettings = async () => {
    try {
      const data = await client.getDataSettings();
      const days = Number(data.retention_days ?? 90);
      setArticleRetentionDays(String(Number.isFinite(days) && days > 0 ? Math.floor(days) : 90));
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const saveDataSettings = async () => {
    const days = Number(articleRetentionDays);
    if (!Number.isInteger(days) || days <= 0 || days > 3650) {
      setMessage("文章保留天数需为 1-3650 的整数", true);
      return;
    }
    try {
      const data = await client.updateDataSettings(days);
      const value = Number(data.retention_days ?? days);
      setArticleRetentionDays(String(value));
      setMessage("数据保留策略已保存");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const refreshFeedsFromNetwork = async () => {
    if (isRefreshingFeeds) {
      return;
    }
    setIsRefreshingFeeds(true);
    setRefreshFailures([]);
    try {
      setMessage("正在远端抓取订阅源...");
      const currentFeeds = await client.listFeeds();
      if (currentFeeds.length === 0) {
        setMessage("暂无订阅源可刷新");
        return;
      }
      const { successCount, failedCount, failures } = await refreshFeedsBatch(currentFeeds, (feedID) => client.refreshFeed(feedID));
      setRefreshFailures(failures);
      await Promise.all([loadFeeds({ silentStatus: true }), loadArticles({ silentStatus: true })]);
      if (failedCount > 0) {
        setMessage(`订阅源刷新完成：成功 ${successCount}，失败 ${failedCount}`);
      } else {
        setRefreshFailures([]);
        setMessage(`订阅源刷新完成：成功 ${successCount}`);
      }
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      setIsRefreshingFeeds(false);
    }
  };

  const handleRefreshArticles = async () => {
    if (isRefreshingArticles) {
      return;
    }
    setIsRefreshingArticles(true);
    setMessage("正在刷新文章...");
    try {
      const data = await loadArticles({ silentStatus: true });
      if (data) {
        setMessage("文章列表已刷新");
      }
    } finally {
      setIsRefreshingArticles(false);
    }
  };

  const handleRefreshFeeds = async () => {
    await loadFeeds();
  };

  const addFeed = async () => {
    const url = feedURL.trim();
    if (!url) {
      setMessage("请输入 RSS/Atom URL", true);
      return;
    }
    try {
      setMessage("正在添加订阅并抓取...");
      await client.createFeed(url, newFeedFolderID);
      setFeedURL("");
      await Promise.all([loadFeeds(), loadFolders(), loadArticles()]);
      setMessage("订阅添加成功");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const selectScriptFeed = (feedID: number | null) => {
    setScriptFeedID(feedID);
    setScriptDirty(false);
    if (feedID == null) {
      setScriptContent("");
      setScriptLang("shell");
      return;
    }
    const feed = feeds.find((item) => item.id === feedID);
    setScriptContent(feed?.custom_script || "");
    setScriptLang(normalizeScriptLang(feed?.custom_script_lang));
  };

  const saveFeedScript = async () => {
    if (scriptFeedID == null) {
      setMessage("请先选择订阅源", true);
      return;
    }
    try {
      await client.updateFeedScript(scriptFeedID, scriptContent, scriptLang);
      setScriptDirty(false);
      await loadFeeds();
      setMessage("脚本已保存");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const uploadScriptFile = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    const text = await file.text();
    const detectedLang = scriptLangByFileName(file.name);
    setScriptContent(text);
    if (detectedLang) {
      setScriptLang(detectedLang);
    }
    setScriptDirty(true);
    event.target.value = "";
  };

  const selectArticle = async (id: number) => {
    try {
      const article = await client.getArticle(id);
      if (article.is_read) {
        setSelectedArticle(article);
        pushArticleRoute(article.id);
        setMessage(`已打开文章 #${id}`);
        return;
      }

      const updated = await client.setArticleRead(id, true);
      setSelectedArticle(updated);
      pushArticleRoute(updated.id);
      setArticles((current) => current.map((entry) => (entry.id === id ? { ...entry, is_read: true } : entry)));
      setMessage(`已打开文章 #${id}（已自动标记已读）`);
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

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
      setArticles((current) => current.map((entry) => (entry.id === updated.id ? { ...entry, is_favorite: updated.is_favorite, favorited_at: updated.favorited_at } : entry)));
      setMessage(updated.is_favorite ? "已加入收藏" : "已取消收藏");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const openSourceWebsite = () => {
    if (!selectedArticleOpenURL) {
      return;
    }
    window.open(selectedArticleOpenURL, "_blank", "noopener,noreferrer");
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

  const { pushArticleRoute, clearArticleRoute } = useArticleRoute(selectedArticle?.id ?? null, (id) => {
    void selectArticle(id);
  });

  const handleReadFilterChange = (value: ReadFilter) => {
    setReadFilter(value);
    setBufferedCount(PREFETCH_BATCH_SIZE);
    setVisibleCount(VISIBLE_STEP_SIZE);
    rebuildStickyUnreadIDs(articles, selectedFeedID, selectedFolderID, value);
  };

  const handleSortModeChange = (value: SortMode) => {
    setSortMode(value);
    setBufferedCount(PREFETCH_BATCH_SIZE);
    setVisibleCount(VISIBLE_STEP_SIZE);
  };
  const toggleReadFilter = () => {
    handleReadFilterChange(readFilter === "unread" ? "all" : "unread");
  };
  const toggleSortMode = () => {
    handleSortModeChange(sortMode === "latest" ? "oldest" : "latest");
  };

  const computedSidebarWidth = sidebarCollapsed ? COLLAPSED_SIDEBAR_WIDTH : sidebarWidth;
  const layoutStyle = {
    gridTemplateColumns: isNarrow
      ? "1fr"
      : `${computedSidebarWidth}px ${RESIZER_WIDTH}px ${listWidth}px ${RESIZER_WIDTH}px minmax(0, 1fr)`,
  };

  useEffect(() => {
    const onResize = () => {
      setIsNarrow(window.innerWidth <= 900);
    };
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  const beginResize = (target: ResizeTarget) => (event: React.MouseEvent<HTMLDivElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const initialSidebarWidth = sidebarWidth;
    const initialListWidth = listWidth;

    const onMouseMove = (moveEvent: MouseEvent) => {
      const delta = moveEvent.clientX - startX;

      if (target === "sidebar") {
        if (sidebarCollapsed) {
          return;
        }
        const next = Math.min(MAX_SIDEBAR_WIDTH, Math.max(MIN_SIDEBAR_WIDTH, initialSidebarWidth + delta));
        setSidebarWidth(next);
        return;
      }

      const next = Math.min(MAX_LIST_WIDTH, Math.max(MIN_LIST_WIDTH, initialListWidth + delta));
      setListWidth(next);
    };

    const onMouseUp = () => {
      window.removeEventListener("mousemove", onMouseMove);
      window.removeEventListener("mouseup", onMouseUp);
    };

    window.addEventListener("mousemove", onMouseMove);
    window.addEventListener("mouseup", onMouseUp);
  };

  const toggleSettings = () => {
    setSettingsOpen((v) => !v);
  };

  const switchSidebarMode = (mode: SidebarMode) => {
    setSidebarMode(mode);
    if (mode === "favorites") {
      setSelectedFeedID(null);
      setSelectedFolderID(null);
      setBufferedCount(PREFETCH_BATCH_SIZE);
      setVisibleCount(VISIBLE_STEP_SIZE);
      rebuildStickyUnreadIDs(articles, null, null, readFilter);
    }
  };

  const selectFeed = async (feedID: number | null) => {
    const data = await loadArticles();
    const source = data ?? articles;
    const nextFeedID = feedID;
    setSidebarMode("subscriptions");
    setSelectedFeedID(nextFeedID);
    setSelectedFolderID(null);
    setBufferedCount(PREFETCH_BATCH_SIZE);
    setVisibleCount(VISIBLE_STEP_SIZE);
    rebuildStickyUnreadIDs(source, nextFeedID, null, readFilter);
    if (feedID != null) {
      selectScriptFeed(feedID);
    }
  };

  const selectFolder = async (folderID: number | null) => {
    const data = await loadArticles();
    const source = data ?? articles;
    const nextFolderID = folderID;
    setSidebarMode("subscriptions");
    setSelectedFolderID(nextFolderID);
    setSelectedFeedID(null);
    setBufferedCount(PREFETCH_BATCH_SIZE);
    setVisibleCount(VISIBLE_STEP_SIZE);
    rebuildStickyUnreadIDs(source, null, nextFolderID, readFilter);
  };

  useEffect(() => {
    const bootstrap = async () => {
      const [, , loadedArticles] = await Promise.all([loadFeeds(), loadFolders(), loadArticles()]);
      rebuildStickyUnreadIDs(loadedArticles ?? [], selectedFeedID, selectedFolderID, readFilter);
      await Promise.all([loadNetworkSettings(), loadAISettings(), loadDataSettings()]);
    };
    void bootstrap();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (selectedFeedID != null && !feeds.some((feed) => feed.id === selectedFeedID)) {
      setSelectedFeedID(null);
    }
    if (selectedFolderID != null && !folders.some((folder) => folder.id === selectedFolderID)) {
      setSelectedFolderID(null);
    }
  }, [feeds, folders, selectedFeedID, selectedFolderID]);

  useEffect(() => {
    if (selectedArticle && !articles.some((article) => article.id === selectedArticle.id)) {
      setSelectedArticle(null);
      clearArticleRoute();
    }
  }, [articles, selectedArticle, clearArticleRoute]);

  useEffect(() => {
    if (feeds.length === 0) {
      setScriptFeedID(null);
      setScriptContent("");
      return;
    }
    if (scriptFeedID == null || !feeds.some((feed) => feed.id === scriptFeedID)) {
      const nextID = selectedFeedID ?? feeds[0].id;
      setScriptFeedID(nextID);
      const nextFeed = feeds.find((feed) => feed.id === nextID);
      setScriptContent(nextFeed?.custom_script || "");
      setScriptLang(normalizeScriptLang(nextFeed?.custom_script_lang));
      setScriptDirty(false);
      return;
    }
    if (scriptDirty) {
      return;
    }
    const current = feeds.find((feed) => feed.id === scriptFeedID);
    if (current) {
      setScriptContent(current.custom_script || "");
      setScriptLang(normalizeScriptLang(current.custom_script_lang));
    }
  }, [feeds, selectedFeedID, scriptFeedID, scriptDirty]);

  useEffect(() => {
    setBufferedCount((count) => Math.min(Math.max(PREFETCH_BATCH_SIZE, count), Math.max(PREFETCH_BATCH_SIZE, filteredAndSortedArticles.length)));
    setVisibleCount((count) => Math.min(Math.max(VISIBLE_STEP_SIZE, count), filteredAndSortedArticles.length || VISIBLE_STEP_SIZE));
  }, [selectedFeedID, selectedFolderID, readFilter, sortMode, filteredAndSortedArticles.length]);

  useEffect(() => {
    setCollapsedFolders((current) => {
      const next: Record<number, boolean> = {};
      folders.forEach((folder) => {
        if (current[folder.id]) {
          next[folder.id] = true;
        }
      });
      return next;
    });
  }, [folders]);

  useEffect(() => {
    const closeMenu = () => {
      setFolderContextMenu(null);
      setFeedContextMenu(null);
    };
    window.addEventListener("click", closeMenu);
    return () => window.removeEventListener("click", closeMenu);
  }, []);

  useEffect(
    () => () => {
      if (bounceTimerRef.current != null) {
        window.clearTimeout(bounceTimerRef.current);
      }
    },
    [],
  );

  const openFolderContextMenu = (event: React.MouseEvent, folder: Folder) => {
    event.preventDefault();
    event.stopPropagation();
    const menuWidth = 188;
    const menuHeight = 170;
    const x = Math.max(10, Math.min(event.clientX, window.innerWidth - menuWidth - 10));
    const y = Math.max(10, Math.min(event.clientY, window.innerHeight - menuHeight - 10));
    setFolderContextMenu({
      folder,
      x,
      y,
    });
    setFeedContextMenu(null);
  };

  const openFeedContextMenu = (event: React.MouseEvent, feed: Feed) => {
    event.preventDefault();
    event.stopPropagation();
    const menuWidth = 196;
    const menuHeight = 206;
    const x = Math.max(10, Math.min(event.clientX, window.innerWidth - menuWidth - 10));
    const y = Math.max(10, Math.min(event.clientY, window.innerHeight - menuHeight - 10));
    setFeedContextMenu({
      feed,
      x,
      y,
    });
    setFolderContextMenu(null);
  };

  const createSubFolder = async () => {
    if (!folderContextMenu) return;
    const name = window.prompt("输入子分类名称");
    if (!name) return;
    try {
      await client.createFolder(name, folderContextMenu.folder.id);
      await loadFolders();
      setMessage("子分类已创建");
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      setFolderContextMenu(null);
    }
  };

  const renameFolder = async () => {
    if (!folderContextMenu) return;
    const current = folderContextMenu.folder;
    const name = window.prompt("输入新的分类名称", current.name);
    if (!name) return;
    try {
      await client.updateFolder(current.id, name, current.parent_id ?? null);
      await loadFolders();
      setMessage("分类已重命名");
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      setFolderContextMenu(null);
    }
  };

  const deleteFolder = async () => {
    if (!folderContextMenu) return;
    if (!window.confirm(`确认删除分类「${folderContextMenu.folder.name}」？`)) return;
    try {
      await client.deleteFolder(folderContextMenu.folder.id);
      if (selectedFolderID === folderContextMenu.folder.id) {
        setSelectedFolderID(null);
      }
      await Promise.all([loadFolders(), loadFeeds(), loadArticles()]);
      setMessage("分类已删除");
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      setFolderContextMenu(null);
    }
  };

  const createRootFolder = async () => {
    const name = window.prompt("输入分类名称");
    if (!name) return;
    try {
      const folder = await client.createFolder(name, null);
      await loadFolders();
      setNewFeedFolderID(folder.id);
      setMessage("分类已创建");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const deleteFeed = async (feed: Feed) => {
    try {
      await client.deleteFeed(feed.id);
      if (selectedFeedID === feed.id) {
        setSelectedFeedID(null);
      }
      await Promise.all([loadFeeds(), loadArticles()]);
      setMessage("订阅源已删除");
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      setPendingDeleteFeed(null);
    }
  };

  const startRenameFeed = (feed: Feed) => {
    const original = feed.title || feed.url || "";
    setRenamingFeedID(feed.id);
    setRenamingFeedTitle(original);
    setRenamingFeedOriginalTitle(original);
    setFeedContextMenu(null);
  };

  const clearRenameFeed = () => {
    setRenamingFeedID(null);
    setRenamingFeedTitle("");
    setRenamingFeedOriginalTitle("");
  };

  const renameFeed = async (feedID: number) => {
    if (renamingFeedID !== feedID) {
      return;
    }
    const currentName = feedNameByID.get(feedID) || "";
    const nextTitle = renamingFeedTitle.trim();
    const targetTitle = nextTitle === "" ? renamingFeedOriginalTitle : nextTitle;
    clearRenameFeed();
    if (targetTitle === currentName || targetTitle === "") {
      return;
    }
    try {
      await client.updateFeedTitle(feedID, targetTitle);
      await loadFeeds();
      setMessage("订阅源已重命名");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const openFeedCategoryDialog = (feed: Feed) => {
    setManageCategoryFeed(feed);
    setManageCategoryFolderID(feed.folder_id ?? null);
    setFeedContextMenu(null);
  };

  const saveFeedCategory = async () => {
    if (!manageCategoryFeed) {
      return;
    }
    try {
      await client.updateFeedFolder(manageCategoryFeed.id, manageCategoryFolderID);
      await Promise.all([loadFeeds(), loadArticles()]);
      setMessage("订阅分类已更新");
      setManageCategoryFeed(null);
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const openScriptSettingsForFeed = (feed: Feed) => {
    selectScriptFeed(feed.id);
    setSettingsTab("script");
    setSettingsOpen(true);
    setFeedContextMenu(null);
  };

  const downloadBlob = (blob: Blob, fileName: string) => {
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = fileName;
    a.click();
    URL.revokeObjectURL(url);
  };

  const exportProfileJSON = async () => {
    try {
      const blob = await client.exportProfile();
      downloadBlob(blob, "zflow-profile.json");
      setMessage("已导出个人配置");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const exportOPML = async () => {
    try {
      const blob = await client.exportOPML();
      downloadBlob(blob, "zflow-subscriptions.opml");
      setMessage("已导出 OPML");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const importProfileJSON = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    try {
      const raw = await file.text();
      const result = await client.importProfile(raw);
      await Promise.all([loadFolders(), loadFeeds(), loadArticles()]);
      setMessage(`个人配置导入完成：新增订阅 ${result.imported_feeds ?? 0}，更新订阅 ${result.updated_feeds ?? 0}`);
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      event.target.value = "";
    }
  };

  const importOPML = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    try {
      const raw = await file.text();
      const result = await client.importOPML(raw);
      await Promise.all([loadFolders(), loadFeeds(), loadArticles()]);
      setMessage(`OPML 导入完成：新增订阅 ${result.imported_feeds ?? 0}，更新订阅 ${result.updated_feeds ?? 0}`);
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      event.target.value = "";
    }
  };

  const toggleFolderCollapsed = (folderID: number) => {
    setCollapsedFolders((current) => ({
      ...current,
      [folderID]: !current[folderID],
    }));
  };

  const moveFeedToFolder = async (feedID: number, folderID: number | null) => {
    try {
      await client.updateFeedFolder(feedID, folderID);
      await Promise.all([loadFeeds(), loadArticles()]);
      setMessage(folderID == null ? "订阅已移动到未分类" : "订阅分类已更新");
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      setDragOverFolderID(null);
      setDragOverUncategorized(false);
      setDraggingFeedID(null);
    }
  };

  const onFeedDragStart = (event: React.DragEvent<HTMLButtonElement>, feedID: number) => {
    event.dataTransfer.setData("text/feed-id", String(feedID));
    event.dataTransfer.effectAllowed = "move";
    setDraggingFeedID(feedID);
  };

  const onFeedDragEnd = () => {
    setDraggingFeedID(null);
    setDragOverFolderID(null);
    setDragOverUncategorized(false);
    setDragOverDeleteZone(false);
  };

  const onFolderDragOver = (event: React.DragEvent, folderID: number) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
    setDragOverUncategorized(false);
    setDragOverFolderID(folderID);
  };

  const onFolderDrop = async (event: React.DragEvent, folderID: number) => {
    event.preventDefault();
    const feedID = Number(event.dataTransfer.getData("text/feed-id"));
    if (!Number.isFinite(feedID) || !Number.isInteger(feedID)) {
      return;
    }
    await moveFeedToFolder(feedID, folderID);
  };

  const onUncategorizedDragOver = (event: React.DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
    setDragOverFolderID(null);
    setDragOverUncategorized(true);
    setDragOverDeleteZone(false);
  };

  const onUncategorizedDrop = async (event: React.DragEvent) => {
    event.preventDefault();
    const feedID = Number(event.dataTransfer.getData("text/feed-id"));
    if (!Number.isFinite(feedID) || !Number.isInteger(feedID)) {
      return;
    }
    await moveFeedToFolder(feedID, null);
  };

  const onDeleteZoneDragOver = (event: React.DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
    setDragOverFolderID(null);
    setDragOverUncategorized(false);
    setDragOverDeleteZone(true);
  };

  const onDeleteZoneDragLeave = () => {
    setDragOverDeleteZone(false);
  };

  const onDeleteZoneDrop = (event: React.DragEvent) => {
    event.preventDefault();
    const feedID = Number(event.dataTransfer.getData("text/feed-id"));
    setDragOverDeleteZone(false);
    setDraggingFeedID(null);
    if (!Number.isFinite(feedID) || !Number.isInteger(feedID)) {
      return;
    }
    const feed = feeds.find((item) => item.id === feedID);
    if (!feed) {
      return;
    }
    setPendingDeleteFeed(feed);
  };

  const triggerListBounce = () => {
    setListBounce(true);
    if (bounceTimerRef.current != null) {
      window.clearTimeout(bounceTimerRef.current);
    }
    bounceTimerRef.current = window.setTimeout(() => setListBounce(false), 260);
  };

  const onArticleListScroll = (event: React.UIEvent<HTMLDivElement>) => {
    const el = event.currentTarget;
    const nearBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 180;
    if (!nearBottom) {
      return;
    }
    const noMoreVisible = pagedArticles.length >= filteredAndSortedArticles.length;
    if (noMoreVisible) {
      if (hasNextArticlePage) {
        void fetchNextArticlePage();
        return;
      }
      triggerListBounce();
      return;
    }
    const now = Date.now();
    if (now - lastLoadAtRef.current < LOAD_MORE_COOLDOWN_MS) {
      return;
    }
    lastLoadAtRef.current = now;
    if (visibleCount < effectiveBufferedCount) {
      setVisibleCount((prevVisible) => Math.min(effectiveBufferedCount, prevVisible + VISIBLE_STEP_SIZE));
      return;
    }
    setBufferedCount((prevBuffered) => {
      const nextBuffered = Math.min(filteredAndSortedArticles.length, prevBuffered + PREFETCH_BATCH_SIZE);
      setVisibleCount((prevVisible) => Math.min(nextBuffered, prevVisible + VISIBLE_STEP_SIZE));
      return nextBuffered;
    });
  };

  const renderFeedNode = (feed: Feed, paddingLeft: number) => {
    const isRenaming = renamingFeedID === feed.id;
    const host = feedHost(feed.url);
    const iconSrc = feed.icon_url ? `${apiBase.replace(/\/$/, "")}${feed.icon_url}` : feedIconURLByHost.get(host) || "";
    return (
    <div key={`feed-${feed.id}`} className={`tree-row feed-row ${isRenaming ? "editing" : ""}`}>
      <button
        className={`item feed-item ${selectedFeedID === feed.id ? "active" : ""} ${draggingFeedID === feed.id ? "dragging" : ""}`}
        style={{ paddingLeft }}
        onClick={() => {
          if (!isRenaming) {
            selectFeed(feed.id);
          }
        }}
        draggable={!isRenaming}
        onDragStart={(event) => {
          if (isRenaming) return;
          onFeedDragStart(event, feed.id);
        }}
        onDragEnd={onFeedDragEnd}
      >
        {isRenaming ? (
          <div className="feed-rename-row" onClick={(event) => event.stopPropagation()}>
            <input
              className="feed-rename-input"
              value={renamingFeedTitle}
              onChange={(event) => setRenamingFeedTitle(event.target.value)}
              autoFocus
              onBlur={() => {
                void renameFeed(feed.id);
              }}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  void renameFeed(feed.id);
                }
              }}
            />
          </div>
        ) : (
          <div className="feed-title-row">
            {iconSrc ? (
              <>
                <img
                  className="feed-icon"
                  src={iconSrc}
                  alt=""
                  loading="lazy"
                  onError={(event) => {
                    event.currentTarget.style.display = "none";
                    const fallback = event.currentTarget.nextElementSibling as HTMLElement | null;
                    if (fallback) {
                      fallback.style.display = "inline-flex";
                    }
                  }}
                />
                <span className="feed-icon-fallback" style={{ display: "none" }} aria-hidden="true">
                  <RssFallbackIcon />
                </span>
              </>
            ) : (
              <span className="feed-icon-fallback" aria-hidden="true">
                <RssFallbackIcon />
              </span>
            )}
            <strong>{feed.title || "(未命名源)"}</strong>
          </div>
        )}
        <div className="meta">
          {feed.url} · items={feed.item_count} · {feed.last_fetch_status}
          {feed.last_fetch_status === "failed" && feed.last_fetch_error ? ` · 错误: ${feed.last_fetch_error}` : ""}
        </div>
      </button>
      {!isRenaming && (
        <button className="node-action-btn" onClick={(event) => openFeedContextMenu(event, feed)} title="管理订阅源" aria-label={`管理订阅源 ${feed.title || feed.url}`}>
          ⋯
        </button>
      )}
    </div>
    );
  };

  const renderFolderNode = (folder: Folder, depth = 0) => {
    const children = childFoldersByParent.get(folder.id) || [];
    const folderFeeds = feedsByFolder.get(folder.id) || [];
    const hasChildren = children.length > 0 || folderFeeds.length > 0;
    const expanded = !collapsedFolders[folder.id];
    const paddingLeft = 8 + depth * 14;
    return (
      <div key={`folder-${folder.id}`}>
        <div
          className={`tree-row ${selectedFolderID === folder.id ? "active" : ""} ${dragOverFolderID === folder.id ? "drop-target" : ""}`}
          onDragOver={(event) => onFolderDragOver(event, folder.id)}
          onDragLeave={() => setDragOverFolderID((current) => (current === folder.id ? null : current))}
          onDrop={(event) => onFolderDrop(event, folder.id)}
        >
          <button className={`item folder-item ${selectedFolderID === folder.id ? "active" : ""}`} onClick={() => selectFolder(folder.id)} style={{ paddingLeft }}>
            <span
              className={`folder-caret ${expanded ? "expanded" : ""} ${hasChildren ? "" : "disabled"}`}
              onClick={(event) => {
                event.preventDefault();
                event.stopPropagation();
                if (hasChildren) {
                  toggleFolderCollapsed(folder.id);
                }
              }}
            >
              ▸
            </span>
            <span className="folder-name">{folder.name}</span>
          </button>
          <button className="node-action-btn" onClick={(event) => openFolderContextMenu(event, folder)} title="管理分类" aria-label={`管理分类 ${folder.name}`}>
            ⋯
          </button>
        </div>
        <div className={`folder-children ${expanded ? "expanded" : "collapsed"}`}>
          <div className="folder-children-inner">
            {folderFeeds.map((feed) => renderFeedNode(feed, paddingLeft + 18))}
            {children.map((child) => renderFolderNode(child, depth + 1))}
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className="shell">
      <TopBar
        statusText={error || status}
        isError={Boolean(error)}
        isRefreshingArticles={isRefreshingArticles}
        isRefreshingFeeds={isRefreshingFeeds}
        onRefreshArticles={handleRefreshArticles}
        onRefreshFeeds={refreshFeedsFromNetwork}
      />
      <RefreshFailureBanner failures={refreshFailures} onClose={() => setRefreshFailures([])} />

      <main className={`layout ${sidebarCollapsed ? "sidebar-collapsed" : ""}`} style={layoutStyle}>
        <section className={`panel sidebar ${sidebarCollapsed ? "collapsed" : ""}`}>
          <div className="sidebar-header">
            <h2 className={`sidebar-title ${sidebarCollapsed ? "hidden" : ""}`}>内容导航</h2>
            <button
              className="sidebar-toggle"
              onClick={() => setSidebarCollapsed((v) => !v)}
              aria-label={sidebarCollapsed ? "展开侧栏" : "折叠侧栏"}
              title={sidebarCollapsed ? "展开侧栏" : "折叠侧栏"}
            >
              <span className={`chevron ${sidebarCollapsed ? "right" : "left"}`}>⌃</span>
            </button>
          </div>

          {!sidebarCollapsed && (
            <div className="sidebar-content">
              <div className="sidebar-mode-tabs">
                <button className={`sidebar-mode-tab ${sidebarMode === "subscriptions" ? "active" : ""}`} onClick={() => switchSidebarMode("subscriptions")}>
                  订阅源
                </button>
                <button className={`sidebar-mode-tab ${sidebarMode === "favorites" ? "active" : ""}`} onClick={() => switchSidebarMode("favorites")}>
                  收藏
                </button>
              </div>
              {sidebarMode === "subscriptions" ? (
                <>
                  <div className="section-head">
                    <h3 className="section-title">订阅列表</h3>
                    <button className="mini-btn" onClick={createRootFolder}>
                      新建分类
                    </button>
                  </div>
                  <div className="list">
                    <button
                      className={`item feed-item ${selectedFeedID == null && selectedFolderID == null ? "active" : ""}`}
                      onClick={() => {
                        setSelectedFolderID(null);
                        void selectFeed(null);
                      }}
                    >
                      <strong>全部订阅源</strong>
                    </button>
                    {rootFolders.map((folder) => renderFolderNode(folder))}
                    {uncategorizedFeeds.length > 0 && (
                      <div
                        className={`tree-divider ${dragOverUncategorized ? "drop-target" : ""}`}
                        onDragOver={onUncategorizedDragOver}
                        onDragLeave={() => setDragOverUncategorized(false)}
                        onDrop={onUncategorizedDrop}
                      >
                        未分类（可拖拽到这里取消分类）
                      </div>
                    )}
                    {uncategorizedFeeds.map((feed) => renderFeedNode(feed, 8))}
                    {feeds.length === 0 && <div className="item">暂无订阅</div>}
                  </div>
                </>
              ) : (
                <>
                  <div className="section-head">
                    <h3 className="section-title">收藏文章</h3>
                  </div>
                  <div className="list">
                    {favoriteArticles.map((article) => (
                      <button
                        key={`favorite-${article.id}`}
                        className={`item favorite-entry ${selectedArticle?.id === article.id ? "active" : ""}`}
                        onClick={() => {
                          void selectArticle(article.id);
                        }}
                        title={article.title || "(无标题)"}
                      >
                        <span className="favorite-entry-star" aria-hidden="true">
                          ☆
                        </span>
                        <span className="favorite-entry-title">{article.title || "(无标题)"}</span>
                      </button>
                    ))}
                    {favoriteArticles.length === 0 && <div className="item">暂无收藏</div>}
                  </div>
                </>
              )}
            </div>
          )}

          <div className={`sidebar-footer ${sidebarCollapsed ? "collapsed" : ""}`}>
            <button className="settings-entry" onClick={toggleSettings} title="设置" aria-label="打开设置">
              <span className="gear">⚙</span>
              {!sidebarCollapsed && <span>设置</span>}
            </button>
          </div>

        </section>
        {!isNarrow && (
          <div
            className={`resizer ${sidebarCollapsed ? "disabled" : ""}`}
            onMouseDown={beginResize("sidebar")}
            role="separator"
            aria-orientation="vertical"
            aria-label="调整订阅栏宽度"
          />
        )}

        <section className="panel list-panel">
          <div className="list-header">
            <h2>{articleListTitle}</h2>
            <ArticleListToolbar readFilter={readFilter} sortMode={sortMode} onToggleReadFilter={toggleReadFilter} onToggleSortMode={toggleSortMode} />
          </div>
          <div className={`list article-list ${listBounce ? "bounce" : ""}`} onScroll={onArticleListScroll}>
            <ArticleList
              articles={pagedArticles}
              selectedArticleID={selectedArticle?.id ?? null}
              feedByID={feedByID}
              feedNameByID={feedNameByID}
              apiBase={apiBase}
              onSelectArticle={(id) => {
                void selectArticle(id);
              }}
            />
          </div>
          <div className="pager">
            <span className="meta">
              已显示 {pagedArticles.length} / {filteredAndSortedArticles.length} 条 · 预取20条，每次追加10条
            </span>
          </div>

        </section>
        {!isNarrow && (
          <div className="resizer" onMouseDown={beginResize("list")} role="separator" aria-orientation="vertical" aria-label="调整文章列表宽度" />
        )}

        <section className="panel detail-panel">
          <ArticleDetailContent
            key={selectedArticle?.id ?? "empty"}
            article={selectedArticle}
            sanitizedSummaryHTML={sanitizedSummaryHTML}
            sanitizedFullContentHTML={sanitizedFullContentHTML}
            canMarkUnread={Boolean(selectedArticle?.is_read)}
            canToggleFavorite={Boolean(selectedArticle)}
            isFavorite={Boolean(selectedArticle?.is_favorite)}
            canOpenSourceSite={Boolean(selectedArticleOpenURL)}
            canExtractReadable={Boolean(selectedArticle?.link)}
            isExtractingReadable={isExtractingReadable}
            canRefreshArticleCache={Boolean(selectedArticle)}
            isRefreshingArticleCache={isRefreshingArticleCache}
            isTranslatingArticle={isTranslatingArticle}
            sourceSiteURL={selectedArticleOpenURL}
            translationParagraphs={currentTranslationParagraphs}
            onMarkUnread={markUnread}
            onToggleFavorite={toggleFavorite}
            onOpenSourceSite={openSourceWebsite}
            onExtractReadable={extractReadableContent}
            onRefreshArticleCache={refreshCurrentArticleCache}
            onTranslateArticle={translateArticle}
          />
        </section>
      </main>

      {draggingFeedID != null && (
        <div
          className={`delete-dropzone ${dragOverDeleteZone ? "active" : ""}`}
          onDragOver={onDeleteZoneDragOver}
          onDragLeave={onDeleteZoneDragLeave}
          onDrop={onDeleteZoneDrop}
        >
          <div className="delete-dropzone-icon">🗑</div>
          <div className="delete-dropzone-text">拖到这里删除订阅源</div>
        </div>
      )}

      {pendingDeleteFeed && (
        <div className="modal-backdrop" onClick={() => setPendingDeleteFeed(null)}>
          <div className="confirm-modal" onClick={(event) => event.stopPropagation()}>
            <h3>确认删除订阅源</h3>
            <p>{pendingDeleteFeed.title || pendingDeleteFeed.url}</p>
            <div className="row">
              <button className="secondary" onClick={() => setPendingDeleteFeed(null)}>
                取消
              </button>
              <button className="danger-btn" onClick={() => void deleteFeed(pendingDeleteFeed)}>
                删除
              </button>
            </div>
          </div>
        </div>
      )}

      {manageCategoryFeed && (
        <div className="modal-backdrop" onClick={() => setManageCategoryFeed(null)}>
          <div className="confirm-modal category-modal" onClick={(event) => event.stopPropagation()}>
            <h3>修改订阅分类</h3>
            <p>{manageCategoryFeed.title || manageCategoryFeed.url}</p>
            <label htmlFor="manageFeedFolder">目标分类</label>
            <select id="manageFeedFolder" value={manageCategoryFolderID ?? ""} onChange={(e) => setManageCategoryFolderID(e.target.value ? Number(e.target.value) : null)}>
              <option value="">未分类</option>
              {folders.map((folder) => (
                <option key={folder.id} value={folder.id}>
                  {folder.name}
                </option>
              ))}
            </select>
            <div className="row">
              <button className="secondary" onClick={() => setManageCategoryFeed(null)}>
                取消
              </button>
              <button onClick={saveFeedCategory}>保存</button>
            </div>
          </div>
        </div>
      )}

      {settingsOpen && (
        <div className="modal-backdrop settings-backdrop" onClick={() => setSettingsOpen(false)}>
          <div className="settings-modal" onClick={(event) => event.stopPropagation()}>
            <div className="settings-modal-header">
              <h3>设置</h3>
              <button className="sidebar-toggle" onClick={() => setSettingsOpen(false)} aria-label="关闭设置">
                ✕
              </button>
            </div>
            <div className="settings-modal-body">
              <aside className="settings-nav">
                <button className={`settings-tab ${settingsTab === "subscription" ? "active" : ""}`} onClick={() => setSettingsTab("subscription")}>
                  订阅管理
                </button>
                <button className={`settings-tab ${settingsTab === "script" ? "active" : ""}`} onClick={() => setSettingsTab("script")}>
                  脚本设置
                </button>
                <button className={`settings-tab ${settingsTab === "connection" ? "active" : ""}`} onClick={() => setSettingsTab("connection")}>
                  连接设置
                </button>
                <button className={`settings-tab ${settingsTab === "ai" ? "active" : ""}`} onClick={() => setSettingsTab("ai")}>
                  AI 设置
                </button>
                <button className={`settings-tab ${settingsTab === "data" ? "active" : ""}`} onClick={() => setSettingsTab("data")}>
                  数据管理
                </button>
              </aside>
              <section className="settings-page">
                {settingsTab === "subscription" && (
                  <div className="settings-page-inner settings-section-card">
                    <h4 className="section-title">添加订阅</h4>
                    <label htmlFor="feedUrl">RSS/Atom URL</label>
                    <input id="feedUrl" value={feedURL} placeholder="https://example.com/feed.xml" onChange={(e) => setFeedURL(e.target.value)} />
                    <label htmlFor="folderSelect">归类到文件夹</label>
                    <div className="row">
                      <select id="folderSelect" value={newFeedFolderID ?? ""} onChange={(e) => setNewFeedFolderID(e.target.value ? Number(e.target.value) : null)}>
                        <option value="">未分类</option>
                        {folders.map((folder) => (
                          <option key={folder.id} value={folder.id}>
                            {folder.name}
                          </option>
                        ))}
                      </select>
                      <button className="secondary" onClick={createRootFolder}>
                        新建分类
                      </button>
                    </div>
                    <div className="row settings-actions">
                      <button onClick={addFeed}>添加并首抓</button>
                      <button className="secondary" onClick={handleRefreshFeeds}>
                        刷新订阅
                      </button>
                      <button className="secondary" onClick={refreshFeedsFromNetwork} disabled={isRefreshingFeeds || isRefreshingArticles}>
                        远端抓取
                      </button>
                      <button className="secondary" onClick={handleRefreshArticles} disabled={isRefreshingArticles || isRefreshingFeeds}>
                        刷新文章
                      </button>
                    </div>
                  </div>
                )}
                {settingsTab === "script" && (
                  <div className="settings-page-inner settings-section-card">
                    <h4 className="section-title">脚本设置（按订阅源）</h4>
                    <label htmlFor="scriptFeed">订阅源</label>
                    <select id="scriptFeed" value={scriptFeedID ?? ""} onChange={(e) => selectScriptFeed(e.target.value ? Number(e.target.value) : null)}>
                      <option value="">请选择</option>
                      {feeds.map((feed) => (
                        <option key={feed.id} value={feed.id}>
                          {feed.title || feed.url}
                        </option>
                      ))}
                    </select>
                    <label htmlFor="scriptUpload">上传脚本文件</label>
                    <input id="scriptUpload" type="file" accept=".sh,.txt,.js,.py,.rb,.pl,.bash" onChange={uploadScriptFile} />
                    <label htmlFor="scriptLang">脚本语言</label>
                    <select
                      id="scriptLang"
                      value={scriptLang}
                      onChange={(e) => {
                        setScriptLang(e.target.value as ScriptLang);
                        setScriptDirty(true);
                      }}
                    >
                      <option value="shell">shell</option>
                      <option value="python">python</option>
                      <option value="javascript">javascript</option>
                    </select>
                    <label htmlFor="scriptContent">脚本内容（stdin 为 JSON v1，stdout 必须返回 JSON，content_html 为最终全文）</label>
                    <textarea
                      id="scriptContent"
                      className="script-editor"
                      rows={8}
                      value={scriptContent}
                      onChange={(e) => {
                        setScriptContent(e.target.value);
                        setScriptDirty(true);
                      }}
                      placeholder={`#!/bin/sh\n# stdin 为 JSON v1，stdout 返回 JSON（示例）\necho '{"ok":true,"content_html":"<article>...</article>"}'`}
                    />
                    <div className="row">
                      <button className="secondary" onClick={saveFeedScript}>
                        保存脚本
                      </button>
                    </div>
                  </div>
                )}
                {settingsTab === "connection" && (
                  <div className="settings-page-inner settings-section-card">
                    <h4 className="section-title">连接设置</h4>
                    <label htmlFor="apiBase">API Base URL</label>
                    <div className="row">
                      <input id="apiBase" value={apiBase} onChange={(e) => setApiBase(e.target.value)} />
                      <button className="secondary" onClick={handleSaveAPIBase}>
                        保存
                      </button>
                    </div>
                    <label htmlFor="networkProxy">网络代理 URL（可选，支持 http/https/socks5）</label>
                    <div className="row">
                      <input id="networkProxy" value={networkProxyURL} placeholder="http://127.0.0.1:7890" onChange={(e) => setNetworkProxyURL(e.target.value)} />
                      <button className="secondary" onClick={saveNetworkSettings}>
                        保存代理
                      </button>
                    </div>
                  </div>
                )}
                {settingsTab === "ai" && (
                  <div className="settings-page-inner settings-section-card">
                    <h4 className="section-title">AI 设置</h4>
                    <label htmlFor="aiApiKey">API Key</label>
                    <input id="aiApiKey" type="password" value={aiAPIKey} placeholder="sk-..." onChange={(e) => setAIAPIKey(e.target.value)} />
                    <label htmlFor="aiBaseURL">Base URL（OpenAI 兼容）</label>
                    <input id="aiBaseURL" value={aiBaseURL} placeholder="https://api.openai.com/v1" onChange={(e) => setAIBaseURL(e.target.value)} />
                    <label htmlFor="aiModel">模型</label>
                    <input id="aiModel" value={aiModel} placeholder="gpt-4o-mini" onChange={(e) => setAIModel(e.target.value)} />
                    <label htmlFor="aiTargetLang">默认目标语言</label>
                    <input id="aiTargetLang" value={aiTargetLang} placeholder="zh-CN" onChange={(e) => setAITargetLang(e.target.value)} />
                    <div className="row">
                      <button className="secondary" onClick={saveAISettings}>
                        保存 AI 设置
                      </button>
                    </div>
                  </div>
                )}
                {settingsTab === "data" && (
                  <div className="settings-page-inner settings-section-card">
                    <h4 className="section-title">清理策略</h4>
                    <label htmlFor="retentionDays">文章保留天数（收藏不会被清理）</label>
                    <div className="row">
                      <input
                        id="retentionDays"
                        type="number"
                        min={1}
                        max={3650}
                        value={articleRetentionDays}
                        onChange={(e) => setArticleRetentionDays(e.target.value)}
                      />
                      <button className="secondary" onClick={saveDataSettings}>
                        保存策略
                      </button>
                    </div>

                    <h4 className="section-title">数据导出</h4>
                    <div className="row settings-actions">
                      <button onClick={exportProfileJSON}>导出个人配置（JSON）</button>
                      <button className="secondary" onClick={exportOPML}>
                        导出订阅源（OPML）
                      </button>
                    </div>

                    <h4 className="section-title">数据导入</h4>
                    <label htmlFor="importProfile">导入个人配置（JSON，含分类与脚本）</label>
                    <input id="importProfile" type="file" accept=".json,application/json" onChange={importProfileJSON} />

                    <label htmlFor="importOPML">导入订阅源（OPML）</label>
                    <input id="importOPML" type="file" accept=".opml,.xml,text/xml,application/xml" onChange={importOPML} />
                  </div>
                )}
              </section>
            </div>
          </div>
        </div>
      )}

      {feedContextMenu && (
        <div className="context-menu" style={{ left: feedContextMenu.x, top: feedContextMenu.y }} onClick={(e) => e.stopPropagation()}>
          <button className="context-item" onClick={() => startRenameFeed(feedContextMenu.feed)}>
            重命名订阅
          </button>
          <button className="context-item" onClick={() => openFeedCategoryDialog(feedContextMenu.feed)}>
            修改分类
          </button>
          <button className="context-item" onClick={() => openScriptSettingsForFeed(feedContextMenu.feed)}>
            设置脚本
          </button>
          <button
            className="context-item danger"
            onClick={() => {
              setPendingDeleteFeed(feedContextMenu.feed);
              setFeedContextMenu(null);
            }}
          >
            删除订阅
          </button>
        </div>
      )}

      {folderContextMenu && (
        <div className="context-menu" style={{ left: folderContextMenu.x, top: folderContextMenu.y }} onClick={(e) => e.stopPropagation()}>
          <button className="context-item" onClick={createSubFolder}>
            新建子分类
          </button>
          <button className="context-item" onClick={renameFolder}>
            重命名分类
          </button>
          <button className="context-item danger" onClick={deleteFolder}>
            删除分类
          </button>
        </div>
      )}
    </div>
  );
}
