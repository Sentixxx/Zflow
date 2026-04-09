import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Article, Feed, Folder } from "@/types";
import { SORT_MODE_LABELS, filterAndSortArticles } from "@/lib/article-list";
import type { ReadFilter, SortMode } from "@/lib/article-list";
import { sanitizeRichHTML } from "@/lib/sanitize";
import { buildTranslationTemplate } from "@/lib/translation";
import { buildFeedIconURLByHost } from "@/lib/feed-utils";
import { resolveInitialAPIBase } from "@/lib/api-base";
import { buildDescendantFolderIDs } from "@/lib/folder-tree";
import { canRequestReadability, shouldAutoFetchReadability } from "@/lib/readability";
import {
  TopBar,
  RefreshFailureBanner,
  ArticleListToolbar,
  ArticleList,
  ArticleDetailContent,
  SidebarTree,
  SettingsModal,
} from "@/components";
import type { SettingsTab } from "@/components";
import { useReaderStore } from "@/stores/useReaderStore";
import { useArticleRoute } from "@/hooks/useArticleRoute";
import { useReaderBootstrap } from "@/hooks/useReaderBootstrap";
import { useArticleActions } from "@/hooks/useArticleActions";
import type { TranslationParagraph } from "@/hooks/useArticleActions";
import { useSettingsActions } from "@/hooks/useSettingsActions";
import { useSidebarFeedActions } from "@/hooks/useSidebarFeedActions";
import { useReaderLayout } from "@/hooks/useReaderLayout";
import { shouldHydrateArticlePool } from "@/hooks/article-pages";
import { useSettingsState } from "@/hooks/useSettingsState";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const PREFETCH_BATCH_SIZE = 20;
const VISIBLE_STEP_SIZE = 10;
const LOAD_MORE_COOLDOWN_MS = 80;
type SidebarMode = "subscriptions" | "favorites";
type MobilePane = "nav" | "list" | "detail";
type ReaderPageProps = {
  initialSettingsOpen?: boolean;
};

function toValidURL(raw: string | undefined): string {
  if (!raw) return "";
  try {
    return new URL(raw).toString();
  } catch {
    return "";
  }
}

export function ReaderPage({ initialSettingsOpen = false }: ReaderPageProps) {
  const [apiBase, setApiBase] = useState<string>(() => resolveInitialAPIBase(localStorage.getItem("zflow_api_base"), window.location.hostname));
  const settingsState = useSettingsState();
  const [selectedArticle, setSelectedArticle] = useState<Article | null>(null);
  const selectedFeedID = useReaderStore((state) => state.selectedFeedID);
  const setSelectedFeedID = useReaderStore((state) => state.setSelectedFeedID);
  const selectedFolderID = useReaderStore((state) => state.selectedFolderID);
  const setSelectedFolderID = useReaderStore((state) => state.setSelectedFolderID);
  const readFilter = useReaderStore((state) => state.readFilter);
  const setReadFilter = useReaderStore((state) => state.setReadFilter);
  const sortMode = useReaderStore((state) => state.sortMode);
  const setSortMode = useReaderStore((state) => state.setSortMode);
  const [bufferedCount, setBufferedCount] = useState<number>(PREFETCH_BATCH_SIZE);
  const [visibleCount, setVisibleCount] = useState<number>(VISIBLE_STEP_SIZE);
  const [stickyUnreadIDs, setStickyUnreadIDs] = useState<number[]>([]);
  const [sidebarCollapsed, setSidebarCollapsed] = useState<boolean>(false);
  const [settingsOpen, setSettingsOpen] = useState<boolean>(initialSettingsOpen);
  const [settingsTab, setSettingsTab] = useState<SettingsTab>("subscription");
  const [sidebarMode, setSidebarMode] = useState<SidebarMode>("subscriptions");
  const [listBounce, setListBounce] = useState<boolean>(false);
  const [isExtractingReadable, setIsExtractingReadable] = useState<boolean>(false);
  const [isRefreshingArticleCache, setIsRefreshingArticleCache] = useState<boolean>(false);
  const [translationParagraphsByArticleID, setTranslationParagraphsByArticleID] = useState<Record<number, TranslationParagraph[]>>({});
  const [translationVisibleByArticleID, setTranslationVisibleByArticleID] = useState<Record<number, boolean>>({});
  const [translationRunningByArticleID, setTranslationRunningByArticleID] = useState<Record<number, boolean>>({});
  const [mobilePane, setMobilePane] = useState<MobilePane>("list");
  const lastLoadAtRef = useRef<number>(0);
  const bounceTimerRef = useRef<number | null>(null);
  const autoReadableAttemptedRef = useRef<Set<number>>(new Set());
  const {
    networkProxyURL,
    aiProtocol,
    aiAPIKey,
    aiAPIKeyMasked,
    aiAPIKeyConfigured,
    aiBaseURL,
    aiModel,
    aiTargetLang,
    embeddingAPIKey,
    embeddingAPIKeyMasked,
    embeddingAPIKeyConfigured,
    embeddingBaseURL,
    embeddingModel,
    articleRetentionDays,
    scriptFeedID,
    scriptContent,
    scriptLang,
    scriptDirty,
    setNetworkProxyURL,
    setAIProtocol,
    setAIAPIKey,
    setAIAPIKeyMasked,
    setAIAPIKeyConfigured,
    setAIBaseURL,
    setAIModel,
    setAITargetLang,
    setEmbeddingAPIKey,
    setEmbeddingAPIKeyMasked,
    setEmbeddingAPIKeyConfigured,
    setEmbeddingBaseURL,
    setEmbeddingModel,
    setArticleRetentionDays,
    setScriptFeedID,
    setScriptContent,
    setScriptLang,
    setScriptDirty,
  } = settingsState;
  const {
    client,
    feeds,
    folders,
    loadFeeds,
    loadFolders,
    articles,
    setArticles,
    loadArticles,
    fetchNextArticlePage,
    hasNextArticlePage,
    feedURL,
    setFeedURL,
    newFeedFolderID,
    setNewFeedFolderID,
    isRefreshingFeeds,
    isRefreshingArticles,
    refreshFailures,
    setRefreshFailures,
    refreshFeedsFromNetwork,
    handleRefreshArticles,
    handleRefreshFeeds,
    addFeed,
    createRootFolder,
    setMessage,
    status,
    error,
    articlesInfiniteQuery,
  } = useReaderBootstrap(apiBase, sortMode);
  const sanitizedSummaryHTML = useMemo(
    () => sanitizeRichHTML(selectedArticle?.display_summary || selectedArticle?.summary),
    [selectedArticle?.display_summary, selectedArticle?.summary],
  );
  const sanitizedFullContentHTML = useMemo(() => sanitizeRichHTML(selectedArticle?.full_content), [selectedArticle?.full_content]);
  const translationTemplate = useMemo(
    () => buildTranslationTemplate(sanitizedFullContentHTML),
    [sanitizedFullContentHTML],
  );
  const translationSources = translationTemplate.sources;


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
  const descendantFolderIDs = useMemo(() => buildDescendantFolderIDs(folders), [folders]);
  const filterArticlesByScope = useCallback(
    (items: Article[], feedID: number | null, folderID: number | null): Article[] => {
      if (sidebarMode === "favorites") {
        return items.filter((article) => article.is_favorite);
      }
      if (feedID != null) {
        return items.filter((article) => article.feed_id === feedID);
      }
      if (folderID != null) {
        const allowed = descendantFolderIDs.get(folderID);
        if (!allowed || allowed.size === 0) {
          return [];
        }
        return items.filter((article) => {
          const feedFolderID = feedByID.get(article.feed_id)?.folder_id;
          return feedFolderID != null && allowed.has(feedFolderID);
        });
      }
      return items;
    },
    [sidebarMode, descendantFolderIDs, feedByID],
  );
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
    return filterAndSortArticles(filteredBySource, readFilter, sortMode, new Set(stickyUnreadIDs), true);
  }, [articles, readFilter, sortMode, selectedFeedID, selectedFolderID, stickyUnreadIDs, filterArticlesByScope]);
  const effectiveBufferedCount = Math.min(bufferedCount, filteredAndSortedArticles.length);
  const pagedArticles = useMemo(
    () => filteredAndSortedArticles.slice(0, Math.min(visibleCount, effectiveBufferedCount)),
    [filteredAndSortedArticles, visibleCount, effectiveBufferedCount],
  );
  const selectedArticleIndex = useMemo(() => {
    if (!selectedArticle) {
      return -1;
    }
    return filteredAndSortedArticles.findIndex((article) => article.id === selectedArticle.id);
  }, [filteredAndSortedArticles, selectedArticle]);
  const previousArticleID = selectedArticleIndex > 0 ? filteredAndSortedArticles[selectedArticleIndex - 1].id : null;
  const nextArticleID =
    selectedArticleIndex >= 0 && selectedArticleIndex < filteredAndSortedArticles.length - 1 ? filteredAndSortedArticles[selectedArticleIndex + 1].id : null;
  const detailProgressText = selectedArticleIndex >= 0 ? `第 ${selectedArticleIndex + 1} / ${filteredAndSortedArticles.length} 条` : "";
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
  const hasListContextOverrides = sidebarMode !== "subscriptions" || selectedFeedID != null || selectedFolderID != null || readFilter !== "all" || sortMode !== "latest";
  const listContextSummary = useMemo(() => {
    let scopeLabel = "全部文章";
    if (sidebarMode === "favorites") {
      scopeLabel = "收藏文章";
    } else if (selectedFeedID != null) {
      scopeLabel = `订阅：${feedNameByID.get(selectedFeedID) || `#${selectedFeedID}`}`;
    } else if (selectedFolderID != null) {
      scopeLabel = `分类：${folderNameByID.get(selectedFolderID) || `#${selectedFolderID}`}`;
    }
    const readLabel = readFilter === "unread" ? "仅未读" : "含已读";
    const sortLabel = SORT_MODE_LABELS[sortMode];
    return `${scopeLabel} · ${readLabel} · ${sortLabel}`;
  }, [sidebarMode, selectedFeedID, selectedFolderID, feedNameByID, folderNameByID, readFilter, sortMode]);
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
  const canExtractReadable = canRequestReadability(selectedArticle);
  const currentTranslationParagraphs = useMemo(() => {
    if (!selectedArticle) {
      return [];
    }
    return translationParagraphsByArticleID[selectedArticle.id] || [];
  }, [selectedArticle, translationParagraphsByArticleID]);
  const isCurrentTranslationVisible = useMemo(() => {
    if (!selectedArticle) {
      return false;
    }
    return translationVisibleByArticleID[selectedArticle.id] ?? false;
  }, [selectedArticle, translationVisibleByArticleID]);
  const isCurrentTranslationRunning = useMemo(() => {
    if (!selectedArticle) {
      return false;
    }
    return translationRunningByArticleID[selectedArticle.id] ?? false;
  }, [selectedArticle, translationRunningByArticleID]);

  const {
    handleSaveAPIBase,
    loadNetworkSettings,
    saveNetworkSettings,
    loadAISettings,
    saveAISettings,
    loadDataSettings,
    saveDataSettings,
    regenerateSummaries,
    refreshCurrentArticleAISummary,
    clearCurrentArticleAISummary,
    clearRecentAISummaries,
    isRefreshingCurrentArticleAISummary,
    selectScriptFeed,
    saveFeedScript,
    uploadScriptFile,
    exportProfileJSON,
    exportOPML,
    importProfileJSON,
    importOPML,
  } = useSettingsActions({
    client,
    feeds,
    selectedArticleID: selectedArticle?.id ?? null,
    selectedArticleTitle: selectedArticle?.title ?? "",
    selectedArticleSummaryStatus: selectedArticle?.display_summary_status ?? "",
    apiBase,
    networkProxyURL,
    aiProtocol,
    aiAPIKey,
    aiBaseURL,
    aiModel,
    aiTargetLang,
    embeddingAPIKey,
    embeddingBaseURL,
    embeddingModel,
    articleRetentionDays,
    scriptFeedID,
    scriptContent,
    scriptLang,
    setNetworkProxyURL,
    setAIProtocol,
    setAIAPIKey,
    setAIAPIKeyMasked,
    setAIAPIKeyConfigured,
    setAIBaseURL,
    setAIModel,
    setAITargetLang,
    setEmbeddingAPIKey,
    setEmbeddingAPIKeyMasked,
    setEmbeddingAPIKeyConfigured,
    setEmbeddingBaseURL,
    setEmbeddingModel,
    setArticleRetentionDays,
    setScriptFeedID,
    setScriptContent,
    setScriptLang,
    setScriptDirty,
    loadFeeds,
    loadFolders,
    loadArticles,
    onSelectedArticleUpdated: setSelectedArticle,
    setMessage,
  });

  const selectArticle = async (id: number) => {
    try {
      const article = await client.getArticle(id);
      if (article.is_read) {
        setSelectedArticle(article);
        pushArticleRoute(article.id);
        if (isNarrow) {
          setMobilePane("detail");
        }
        setMessage(`已打开文章 #${id}`);
        return;
      }

      const updated = await client.setArticleRead(id, true);
      setSelectedArticle(updated);
      pushArticleRoute(updated.id);
      setArticles((current) => current.map((entry) => (entry.id === id ? { ...entry, is_read: true } : entry)));
      if (isNarrow) {
        setMobilePane("detail");
      }
      setMessage(`已打开文章 #${id}（已自动标记已读）`);
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

  const { markUnread, toggleFavorite, extractReadableContent, refreshCurrentArticleCache, translateArticle } = useArticleActions({
    client,
    selectedArticle,
    setSelectedArticle,
    setArticles,
    setMessage,
    isExtractingReadable,
    setIsExtractingReadable,
    isRefreshingArticleCache,
    setIsRefreshingArticleCache,
    aiTargetLang,
    translationSources,
    translationParagraphsByArticleID,
    setTranslationParagraphsByArticleID,
    translationVisibleByArticleID,
    setTranslationVisibleByArticleID,
    translationRunningByArticleID,
    setTranslationRunningByArticleID,
  });

  const { pushArticleRoute, clearArticleRoute } = useArticleRoute(selectedArticle?.id ?? null, (id) => {
    void selectArticle(id);
  });

  useEffect(() => {
    if (!selectedArticle) {
      return;
    }
    const normalizedFullContent = (selectedArticle.full_content || "").trim();
    const looksLikePDFGarbage =
      /^%PDF-\d/i.test(normalizedFullContent) || (normalizedFullContent.includes("xref") && normalizedFullContent.includes("endobj"));
    const hasUsableFullContent = Boolean(normalizedFullContent) && !looksLikePDFGarbage;
    if (!shouldAutoFetchReadability({ article: selectedArticle, hasUsableFullContent, isExtractingReadable })) {
      return;
    }
    if (autoReadableAttemptedRef.current.has(selectedArticle.id)) {
      return;
    }
    autoReadableAttemptedRef.current.add(selectedArticle.id);
    void extractReadableContent();
  }, [selectedArticle?.id, selectedArticle?.link, selectedArticle?.full_content, isExtractingReadable]);

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

  const toggleSettings = () => {
    setSettingsOpen((v) => !v);
  };

  const openQuickAddFeed = () => {
    setSettingsTab("subscription");
    setSettingsOpen(true);
  };

  const switchSidebarMode = (mode: SidebarMode) => {
    setSidebarMode(mode);
    if (isNarrow) {
      setMobilePane("list");
    }
    if (mode === "favorites") {
      setSelectedFeedID(null);
      setSelectedFolderID(null);
      setBufferedCount(PREFETCH_BATCH_SIZE);
      setVisibleCount(VISIBLE_STEP_SIZE);
      rebuildStickyUnreadIDs(articles, null, null, readFilter);
    }
  };

  const selectFeed = (feedID: number | null) => {
    const nextFeedID = feedID;
    setSidebarMode("subscriptions");
    setSelectedFeedID(nextFeedID);
    setSelectedFolderID(null);
    setBufferedCount(PREFETCH_BATCH_SIZE);
    setVisibleCount(VISIBLE_STEP_SIZE);
    rebuildStickyUnreadIDs(articles, nextFeedID, null, readFilter);
    if (isNarrow) {
      setMobilePane("list");
    }
    if (feedID != null) {
      selectScriptFeed(feedID);
    }
  };

  const selectFolder = (folderID: number | null) => {
    const nextFolderID = folderID;
    setSidebarMode("subscriptions");
    setSelectedFolderID(nextFolderID);
    setSelectedFeedID(null);
    setBufferedCount(PREFETCH_BATCH_SIZE);
    setVisibleCount(VISIBLE_STEP_SIZE);
    rebuildStickyUnreadIDs(articles, null, nextFolderID, readFilter);
    if (isNarrow) {
      setMobilePane("list");
    }
  };

  const {
    folderContextMenu,
    feedContextMenu,
    renamingFeedID,
    renamingFeedTitle,
    setRenamingFeedTitle,
    manageCategoryFeed,
    setManageCategoryFeed,
    manageCategoryFolderID,
    setManageCategoryFolderID,
    collapsedFolders,
    draggingFeedID,
    dragOverFolderID,
    dragOverUncategorized,
    dragOverDeleteZone,
    pendingDeleteFeed,
    setPendingDeleteFeed,
    openFolderContextMenu,
    openFeedContextMenu,
    closeFeedContextMenu,
    createSubFolder,
    renameFolder,
    deleteFolder,
    createRootFolder: createSidebarRootFolder,
    deleteFeed,
    startRenameFeed,
    renameFeed,
    openFeedCategoryDialog,
    saveFeedCategory,
    openScriptSettingsForFeed,
    toggleFolderCollapsed,
    onFeedDragStart,
    onFeedDragEnd,
    onFolderDragOver,
    onFolderDragLeave,
    onFolderDrop,
    onUncategorizedDragOver,
    onUncategorizedDragLeave,
    onUncategorizedDrop,
    onDeleteZoneDragOver,
    onDeleteZoneDragLeave,
    onDeleteZoneDrop,
  } = useSidebarFeedActions({
    client,
    folders,
    feeds,
    selectedFeedID,
    selectedFolderID,
    setSelectedFeedID,
    setSelectedFolderID,
    setMessage,
    loadFeeds,
    loadFolders,
    loadArticles,
    feedNameByID,
    setNewFeedFolderID,
    selectScriptFeed,
    setSettingsTab,
    setSettingsOpen,
  });
  const { isNarrow, layoutStyle, beginResize } = useReaderLayout({ sidebarCollapsed });

  useEffect(() => {
    if (!isNarrow) {
      setMobilePane("list");
      return;
    }
    setSidebarCollapsed(false);
    setMobilePane(selectedArticle ? "detail" : "list");
  }, [isNarrow]);

  useEffect(() => {
    if (!isNarrow) {
      return;
    }
    if (mobilePane === "detail" && !selectedArticle) {
      setMobilePane("list");
    }
  }, [isNarrow, mobilePane, selectedArticle]);

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
    if (initialSettingsOpen) {
      setSettingsOpen(true);
    }
  }, [initialSettingsOpen]);

  useEffect(() => {
    if (selectedArticle && !articles.some((article) => article.id === selectedArticle.id)) {
      setSelectedArticle(null);
      clearArticleRoute();
    }
  }, [articles, selectedArticle, clearArticleRoute]);

  useEffect(() => {
    if (!selectedArticle) {
      return;
    }
    const inCurrentList = filteredAndSortedArticles.some((article) => article.id === selectedArticle.id);
    if (inCurrentList) {
      return;
    }
    setSelectedArticle(null);
    clearArticleRoute();
    if (isNarrow) {
      setMobilePane("list");
    }
    setMessage("当前文章不在此列表范围，已返回列表");
  }, [filteredAndSortedArticles, selectedArticle, clearArticleRoute, isNarrow]);

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
      setScriptLang(nextFeed?.custom_script_lang === "python" || nextFeed?.custom_script_lang === "javascript" ? nextFeed.custom_script_lang : "shell");
      setScriptDirty(false);
      return;
    }
    if (scriptDirty) {
      return;
    }
    const current = feeds.find((feed) => feed.id === scriptFeedID);
    if (current) {
      setScriptContent(current.custom_script || "");
      setScriptLang(current.custom_script_lang === "python" || current.custom_script_lang === "javascript" ? current.custom_script_lang : "shell");
    }
  }, [feeds, selectedFeedID, scriptFeedID, scriptDirty]);

  useEffect(() => {
    setBufferedCount((count) => Math.min(Math.max(PREFETCH_BATCH_SIZE, count), Math.max(PREFETCH_BATCH_SIZE, filteredAndSortedArticles.length)));
    setVisibleCount((count) => Math.min(Math.max(VISIBLE_STEP_SIZE, count), filteredAndSortedArticles.length || VISIBLE_STEP_SIZE));
  }, [selectedFeedID, selectedFolderID, readFilter, sortMode, filteredAndSortedArticles.length]);

  useEffect(() => {
    rebuildStickyUnreadIDs(articles, selectedFeedID, selectedFolderID, readFilter);
  }, [articles, selectedFeedID, selectedFolderID, readFilter]);

  useEffect(
    () => () => {
      if (bounceTimerRef.current != null) {
        window.clearTimeout(bounceTimerRef.current);
      }
    },
    [],
  );

  // Lock body scroll when modal dialogs are open
  useEffect(() => {
    const hasDialog = pendingDeleteFeed != null || manageCategoryFeed != null;
    if (hasDialog) {
      document.body.style.overflow = "hidden";
      return () => { document.body.style.overflow = ""; };
    }
  }, [pendingDeleteFeed, manageCategoryFeed]);

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

  useEffect(() => {
    const loadedCount = articles.length;
    if (!shouldHydrateArticlePool({
      loadedCount,
      hasNextPage: Boolean(hasNextArticlePage),
      isFetching: articlesInfiniteQuery.isFetching,
      isFetchingNextPage: articlesInfiniteQuery.isFetchingNextPage,
    })) {
      return;
    }
    const timer = window.setTimeout(() => {
      void fetchNextArticlePage();
    }, 150);
    return () => window.clearTimeout(timer);
  }, [articles.length, hasNextArticlePage, articlesInfiniteQuery.isFetching, articlesInfiniteQuery.isFetchingNextPage, fetchNextArticlePage]);

  return (
    <div className="h-screen flex flex-col overflow-hidden bg-background">
      {/* Skip to content link for keyboard accessibility */}
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-[60] focus:bg-background focus:px-4 focus:py-2 focus:rounded-md focus:ring-2 focus:ring-ring focus:text-foreground"
      >
        跳转到主要内容
      </a>
      {/* Top bar */}
      <div className="px-3 pt-3 pb-0 shrink-0">
        <TopBar
          statusText={error || status}
          isError={Boolean(error)}
          isRefreshingArticles={isRefreshingArticles}
          isRefreshingFeeds={isRefreshingFeeds}
          onRefreshArticles={handleRefreshArticles}
          onRefreshFeeds={refreshFeedsFromNetwork}
        />
      </div>
      <RefreshFailureBanner failures={refreshFailures} onClose={() => setRefreshFailures([])} />

      {/* Three-column resizable layout */}
      <main
        id="main-content"
        className={cn("flex-1 min-h-0 grid overflow-hidden", isNarrow && "!grid-cols-[1fr] pb-12")}
        style={layoutStyle}
      >
        {/* Sidebar */}
        <section
          className={cn(
            "flex flex-col h-full overflow-hidden border-r border-border",
            isNarrow && mobilePane !== "nav" && "hidden"
          )}
          aria-label="订阅源导航"
        >
          {/* Sidebar header */}
          <div className="flex items-center gap-1.5 px-3 py-2 border-b border-border shrink-0">
            {!sidebarCollapsed && (
              <h2 className="text-xs font-bold uppercase tracking-widest text-muted-foreground flex-1 truncate">
                内容导航
              </h2>
            )}
            {!sidebarCollapsed && (
              <button
                className="w-6 h-6 flex items-center justify-center rounded-md text-muted-foreground hover:bg-muted/60 hover:text-foreground text-base leading-none transition-colors cursor-pointer"
                onClick={openQuickAddFeed}
                title="快速添加订阅源"
                aria-label="快速添加订阅源"
              >
                +
              </button>
            )}
            {!isNarrow && (
              <button
                className="w-6 h-6 flex items-center justify-center rounded-md text-muted-foreground hover:bg-muted/60 hover:text-foreground transition-colors cursor-pointer"
                onClick={() => setSidebarCollapsed((v) => !v)}
                aria-label={sidebarCollapsed ? "展开侧栏" : "折叠侧栏"}
                title={sidebarCollapsed ? "展开侧栏" : "折叠侧栏"}
              >
                <span
                  className={cn(
                    "text-xs transition-transform duration-150",
                    sidebarCollapsed ? "rotate-90" : "-rotate-90"
                  )}
                >
                  ⌃
                </span>
              </button>
            )}
          </div>

          {/* Sidebar tree */}
          {!sidebarCollapsed && (
            <SidebarTree
              sidebarMode={sidebarMode}
              selectedFeedID={selectedFeedID}
              selectedFolderID={selectedFolderID}
              selectedArticleID={selectedArticle?.id ?? null}
              rootFolders={rootFolders}
              childFoldersByParent={childFoldersByParent}
              feedsByFolder={feedsByFolder}
              uncategorizedFeeds={uncategorizedFeeds}
              feeds={feeds}
              favoriteArticles={favoriteArticles}
              collapsedFolders={collapsedFolders}
              dragOverFolderID={dragOverFolderID}
              dragOverUncategorized={dragOverUncategorized}
              draggingFeedID={draggingFeedID}
              renamingFeedID={renamingFeedID}
              renamingFeedTitle={renamingFeedTitle}
              apiBase={apiBase}
              feedIconURLByHost={feedIconURLByHost}
              onSwitchSidebarMode={switchSidebarMode}
              onCreateRootFolder={createRootFolder}
              onSelectFeed={selectFeed}
              onSelectFolder={(folderID) => { selectFolder(folderID); }}
              onSelectArticle={(articleID) => { void selectArticle(articleID); }}
              onToggleFolderCollapsed={toggleFolderCollapsed}
              onOpenFeedContextMenu={openFeedContextMenu}
              onOpenFolderContextMenu={openFolderContextMenu}
              onFeedDragStart={onFeedDragStart}
              onFeedDragEnd={onFeedDragEnd}
              onFolderDragOver={onFolderDragOver}
              onFolderDragLeave={onFolderDragLeave}
              onFolderDrop={(event, folderID) => { void onFolderDrop(event, folderID); }}
              onUncategorizedDragOver={onUncategorizedDragOver}
              onUncategorizedDragLeave={onUncategorizedDragLeave}
              onUncategorizedDrop={(event) => { void onUncategorizedDrop(event); }}
              onRenamingFeedTitleChange={setRenamingFeedTitle}
              onRenameFeed={(feedID) => { void renameFeed(feedID); }}
            />
          )}

          {/* Sidebar footer */}
          <div className="shrink-0 border-t border-border p-2 mt-auto space-y-0.5">
            <button
              className={cn(
                "flex items-center gap-2 w-full px-2 py-1.5 rounded-md text-sm text-muted-foreground hover:bg-muted/60 hover:text-foreground transition-colors",
                sidebarCollapsed && "justify-center"
              )}
              onClick={toggleSettings}
              title="设置"
              aria-label="打开设置"
            >
              <span className="text-base leading-none">⚙</span>
              {!sidebarCollapsed && <span>设置</span>}
            </button>
          </div>
        </section>

        {/* Sidebar resizer */}
        {!isNarrow && (
          <div
            className={cn(
              "w-2 cursor-col-resize bg-border/20 hover:bg-primary/20 transition-colors shrink-0",
              sidebarCollapsed && "cursor-default"
            )}
            onMouseDown={beginResize("sidebar")}
            role="separator"
            aria-orientation="vertical"
            aria-label="调整订阅栏宽度"
          />
        )}

        {/* Article list panel */}
        <section
          className={cn(
            "flex flex-col h-full overflow-hidden border-r border-border",
            isNarrow && mobilePane !== "list" && "hidden"
          )}
        >
          <div className="flex items-center justify-between gap-2 px-3 py-2 border-b border-border shrink-0">
            <h2 className="text-sm font-semibold truncate">{articleListTitle}</h2>
            <ArticleListToolbar
              readFilter={readFilter}
              sortMode={sortMode}
              onToggleReadFilter={toggleReadFilter}
              onSortModeChange={handleSortModeChange}
            />
          </div>
          {hasListContextOverrides && (
            <div className="px-3 py-1.5 text-xs text-muted-foreground border-b border-border bg-muted/20 shrink-0">
              {listContextSummary}
            </div>
          )}
          <ArticleList
            articles={pagedArticles}
            selectedArticleID={selectedArticle?.id ?? null}
            feedByID={feedByID}
            feedNameByID={feedNameByID}
            apiBase={apiBase}
            listBounce={listBounce}
            onScroll={onArticleListScroll}
            onSelectArticle={(id) => { void selectArticle(id); }}
          />
          <div className="px-3 py-2 text-xs text-muted-foreground border-t border-border shrink-0">
            已显示 {pagedArticles.length} / {filteredAndSortedArticles.length} 条 · 预取20条，每次追加10条
          </div>
        </section>

        {/* List resizer */}
        {!isNarrow && (
          <div
            className="w-2 cursor-col-resize bg-border/20 hover:bg-primary/20 transition-colors shrink-0"
            onMouseDown={beginResize("list")}
            role="separator"
            aria-orientation="vertical"
            aria-label="调整文章列表宽度"
          />
        )}

        {/* Article detail panel */}
        <section
          className={cn(
            "flex flex-col h-full overflow-hidden",
            isNarrow && mobilePane !== "detail" && "hidden"
          )}
        >
          <ArticleDetailContent
            key={selectedArticle?.id ?? "empty"}
            article={selectedArticle}
            sanitizedSummaryHTML={sanitizedSummaryHTML}
            sanitizedFullContentHTML={sanitizedFullContentHTML}
            translationTemplateHTML={translationTemplate.html}
            canMarkUnread={Boolean(selectedArticle?.is_read)}
            canToggleFavorite={Boolean(selectedArticle)}
            isFavorite={Boolean(selectedArticle?.is_favorite)}
            canOpenSourceSite={Boolean(selectedArticleOpenURL)}
            canExtractReadable={canExtractReadable}
            isExtractingReadable={isExtractingReadable}
            canRefreshArticleCache={Boolean(selectedArticle)}
            isRefreshingArticleCache={isRefreshingArticleCache}
            isTranslatingArticle={isCurrentTranslationRunning}
            isTranslationVisible={isCurrentTranslationVisible}
            sourceSiteURL={selectedArticleOpenURL}
            detailProgressText={detailProgressText}
            translationParagraphs={currentTranslationParagraphs}
            canGoPrev={previousArticleID != null}
            canGoNext={nextArticleID != null}
            onMarkUnread={markUnread}
            onToggleFavorite={toggleFavorite}
            onOpenSourceSite={openSourceWebsite}
            onExtractReadable={extractReadableContent}
            onRefreshArticleCache={refreshCurrentArticleCache}
            onTranslateArticle={translateArticle}
            onGoPrev={() => { if (previousArticleID != null) void selectArticle(previousArticleID); }}
            onGoNext={() => { if (nextArticleID != null) void selectArticle(nextArticleID); }}
            isNarrow={isNarrow}
          />
        </section>
      </main>

      {/* Mobile bottom navigation */}
      {isNarrow && (
        <nav
          className="fixed bottom-0 left-0 right-0 grid grid-cols-3 border-t border-border bg-background/95 backdrop-blur-sm z-40 pb-[env(safe-area-inset-bottom)]"
          aria-label="移动端分栏导航"
        >
          {(["nav", "list", "detail"] as const).map((pane) => {
            const labels = { nav: "导航", list: "列表", detail: "详情" };
            const counts = { nav: feeds.length, list: filteredAndSortedArticles.length, detail: null };
            return (
              <button
                key={pane}
                className={cn(
                  "flex flex-col items-center justify-center gap-0.5 py-2.5 text-xs transition-colors cursor-pointer",
                  mobilePane === pane
                    ? "text-primary font-medium"
                    : "text-muted-foreground hover:text-foreground",
                  pane === "detail" && !selectedArticle && "opacity-40"
                )}
                onClick={() => setMobilePane(pane)}
                disabled={pane === "detail" && !selectedArticle}
              >
                <span>{labels[pane]}</span>
                {counts[pane] != null && (
                  <span className="text-[10px] text-muted-foreground">{counts[pane]}</span>
                )}
              </button>
            );
          })}
        </nav>
      )}

      {/* Drag-to-delete dropzone */}
      {draggingFeedID != null && (
        <div
          className={cn(
            "fixed bottom-20 left-1/2 -translate-x-1/2 flex flex-col items-center gap-2 px-6 py-4 rounded-xl border-2 border-dashed transition-colors z-50",
            dragOverDeleteZone
              ? "border-destructive bg-destructive/15 text-destructive"
              : "border-border bg-background/90 text-muted-foreground"
          )}
          onDragOver={onDeleteZoneDragOver}
          onDragLeave={onDeleteZoneDragLeave}
          onDrop={onDeleteZoneDrop}
        >
          <span className="text-2xl">🗑</span>
          <span className="text-sm font-medium">拖到这里删除订阅源</span>
        </div>
      )}

      {/* Delete feed confirm dialog */}
      {pendingDeleteFeed && (
        <div
          className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4"
          onClick={() => setPendingDeleteFeed(null)}
          onKeyDown={(e) => { if (e.key === "Escape") setPendingDeleteFeed(null); }}
          role="dialog"
          aria-modal="true"
          aria-label="确认删除订阅源"
        >
          <div
            className="bg-background rounded-xl shadow-xl border border-border p-6 w-full max-w-sm space-y-4"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 className="text-base font-semibold">确认删除订阅源</h3>
            <p className="text-sm text-muted-foreground">{pendingDeleteFeed.title || pendingDeleteFeed.url}</p>
            <div className="flex gap-2 justify-end">
              <Button variant="outline" onClick={() => setPendingDeleteFeed(null)}>取消</Button>
              <Button variant="destructive" onClick={() => void deleteFeed(pendingDeleteFeed)}>删除</Button>
            </div>
          </div>
        </div>
      )}

      {/* Change category dialog */}
      {manageCategoryFeed && (
        <div
          className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4"
          onClick={() => setManageCategoryFeed(null)}
          onKeyDown={(e) => { if (e.key === "Escape") setManageCategoryFeed(null); }}
          role="dialog"
          aria-modal="true"
          aria-label="修改订阅分类"
        >
          <div
            className="bg-background rounded-xl shadow-xl border border-border p-6 w-full max-w-sm space-y-4"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 className="text-base font-semibold">修改订阅分类</h3>
            <p className="text-sm text-muted-foreground">{manageCategoryFeed.title || manageCategoryFeed.url}</p>
            <div className="space-y-1.5">
              <label htmlFor="manageFeedFolder" className="text-sm font-medium">目标分类</label>
              <select
                id="manageFeedFolder"
                value={manageCategoryFolderID ?? ""}
                onChange={(e) => setManageCategoryFolderID(e.target.value ? Number(e.target.value) : null)}
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
              >
                <option value="">未分类</option>
                {folders.map((folder) => (
                  <option key={folder.id} value={folder.id}>{folder.name}</option>
                ))}
              </select>
            </div>
            <div className="flex gap-2 justify-end">
              <Button variant="outline" onClick={() => setManageCategoryFeed(null)}>取消</Button>
              <Button onClick={saveFeedCategory}>保存</Button>
            </div>
          </div>
        </div>
      )}

      <SettingsModal
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        settingsTab={settingsTab}
        onSettingsTabChange={setSettingsTab}
        feedURL={feedURL}
        onFeedURLChange={setFeedURL}
        newFeedFolderID={newFeedFolderID}
        onNewFeedFolderIDChange={setNewFeedFolderID}
        folders={folders}
        onCreateRootFolder={createRootFolder}
        onAddFeed={addFeed}
        onRefreshFeeds={handleRefreshFeeds}
        onRefreshFeedsFromNetwork={refreshFeedsFromNetwork}
        onRefreshArticles={handleRefreshArticles}
        isRefreshingFeeds={isRefreshingFeeds}
        isRefreshingArticles={isRefreshingArticles}
        scriptFeedID={scriptFeedID}
        feeds={feeds}
        scriptLang={scriptLang}
        scriptContent={scriptContent}
        onSelectScriptFeed={selectScriptFeed}
        onUploadScriptFile={uploadScriptFile}
        onScriptLangChange={(lang) => { setScriptLang(lang); setScriptDirty(true); }}
        onScriptContentChange={(value) => { setScriptContent(value); setScriptDirty(true); }}
        onSaveFeedScript={saveFeedScript}
        apiBase={apiBase}
        onAPIBaseChange={setApiBase}
        onSaveAPIBase={handleSaveAPIBase}
        networkProxyURL={networkProxyURL}
        onNetworkProxyURLChange={setNetworkProxyURL}
        onSaveNetworkSettings={saveNetworkSettings}
        aiProtocol={aiProtocol}
        aiAPIKey={aiAPIKey}
        aiAPIKeyMasked={aiAPIKeyMasked}
        aiAPIKeyConfigured={aiAPIKeyConfigured}
        aiBaseURL={aiBaseURL}
        aiModel={aiModel}
        aiTargetLang={aiTargetLang}
        embeddingAPIKey={embeddingAPIKey}
        embeddingAPIKeyMasked={embeddingAPIKeyMasked}
        embeddingAPIKeyConfigured={embeddingAPIKeyConfigured}
        embeddingBaseURL={embeddingBaseURL}
        embeddingModel={embeddingModel}
        onAIProtocolChange={setAIProtocol}
        onAIAPIKeyChange={setAIAPIKey}
        onAIBaseURLChange={setAIBaseURL}
        onAIModelChange={setAIModel}
        onAITargetLangChange={setAITargetLang}
        onEmbeddingAPIKeyChange={setEmbeddingAPIKey}
        onEmbeddingBaseURLChange={setEmbeddingBaseURL}
        onEmbeddingModelChange={setEmbeddingModel}
        onSaveAISettings={saveAISettings}
        articleRetentionDays={articleRetentionDays}
        selectedArticleID={selectedArticle?.id ?? null}
        selectedArticleTitle={selectedArticle?.title || ""}
        isRefreshingCurrentArticleAISummary={isRefreshingCurrentArticleAISummary}
        onArticleRetentionDaysChange={setArticleRetentionDays}
        onSaveDataSettings={saveDataSettings}
        onRefreshCurrentArticleAISummary={refreshCurrentArticleAISummary}
        onClearCurrentArticleAISummary={clearCurrentArticleAISummary}
        onClearRecentAISummaries={clearRecentAISummaries}
        onRegenerateSummaries={regenerateSummaries}
        onExportProfileJSON={exportProfileJSON}
        onExportOPML={exportOPML}
        onImportProfileJSON={importProfileJSON}
        onImportOPML={importOPML}
      />

      {/* Feed context menu */}
      {feedContextMenu && (
        <div
          className="fixed z-50 bg-background border border-border rounded-lg shadow-lg py-1 min-w-[148px]"
          style={{ left: feedContextMenu.x, top: feedContextMenu.y }}
          onClick={(e) => e.stopPropagation()}
        >
          <button className="w-full text-left px-3 py-1.5 text-sm hover:bg-muted/60 transition-colors" onClick={() => startRenameFeed(feedContextMenu.feed)}>重命名订阅</button>
          <button className="w-full text-left px-3 py-1.5 text-sm hover:bg-muted/60 transition-colors" onClick={() => openFeedCategoryDialog(feedContextMenu.feed)}>修改分类</button>
          <button className="w-full text-left px-3 py-1.5 text-sm hover:bg-muted/60 transition-colors" onClick={() => openScriptSettingsForFeed(feedContextMenu.feed)}>设置脚本</button>
          <button
            className="w-full text-left px-3 py-1.5 text-sm text-destructive hover:bg-destructive/10 transition-colors"
            onClick={() => { setPendingDeleteFeed(feedContextMenu.feed); closeFeedContextMenu(); }}
          >
            删除订阅
          </button>
        </div>
      )}

      {/* Folder context menu */}
      {folderContextMenu && (
        <div
          className="fixed z-50 bg-background border border-border rounded-lg shadow-lg py-1 min-w-[148px]"
          style={{ left: folderContextMenu.x, top: folderContextMenu.y }}
          onClick={(e) => e.stopPropagation()}
        >
          <button className="w-full text-left px-3 py-1.5 text-sm hover:bg-muted/60 transition-colors" onClick={createSubFolder}>新建子分类</button>
          <button className="w-full text-left px-3 py-1.5 text-sm hover:bg-muted/60 transition-colors" onClick={renameFolder}>重命名分类</button>
          <button className="w-full text-left px-3 py-1.5 text-sm text-destructive hover:bg-destructive/10 transition-colors" onClick={deleteFolder}>删除分类</button>
        </div>
      )}
    </div>
  );
}
