import { useEffect, useMemo, useRef, useState } from "react";
import { ApiClient } from "@/api";
import type { Article, Feed, Folder } from "@/types";
import { filterAndSortArticles } from "@/lib/article-list";
import type { ReadFilter, SortMode } from "@/lib/article-list";
import { sanitizeRichHTML } from "@/lib/sanitize";
import { buildFeedIconURLByHost } from "@/lib/feed-utils";
import {
  TopBar,
  RefreshFailureBanner,
  ArticleListToolbar,
  ArticleList,
  ArticleDetailContent,
  SidebarTree,
  SettingsModal,
} from "@/components";
import type { RefreshFailure } from "@/components";
import type { ScriptLang, SettingsTab } from "@/components";
import { useReaderStore } from "@/stores/useReaderStore";
import { useArticleRoute } from "@/hooks/useArticleRoute";
import { useReaderQueries } from "@/hooks/useReaderQueries";
import { refreshFeedsBatch } from "@/services/feed-refresh-service";
import { useFeeds } from "@/hooks/useFeeds";
import { useEntries } from "@/hooks/useEntries";
import { useArticleActions } from "@/hooks/useArticleActions";
import type { TranslationParagraph } from "@/hooks/useArticleActions";
import { useSettingsActions } from "@/hooks/useSettingsActions";
import { useSidebarFeedActions } from "@/hooks/useSidebarFeedActions";
import { useReaderLayout } from "@/hooks/useReaderLayout";

const DEFAULT_API_BASE = "http://localhost:8080";
const PREFETCH_BATCH_SIZE = 20;
const VISIBLE_STEP_SIZE = 10;
const LOAD_MORE_COOLDOWN_MS = 80;
type SidebarMode = "subscriptions" | "favorites";
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
  const [settingsOpen, setSettingsOpen] = useState<boolean>(initialSettingsOpen);
  const [settingsTab, setSettingsTab] = useState<SettingsTab>("subscription");
  const [sidebarMode, setSidebarMode] = useState<SidebarMode>("subscriptions");
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

  const {
    handleSaveAPIBase,
    loadNetworkSettings,
    saveNetworkSettings,
    loadAISettings,
    saveAISettings,
    loadDataSettings,
    saveDataSettings,
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
    apiBase,
    networkProxyURL,
    aiAPIKey,
    aiBaseURL,
    aiModel,
    aiTargetLang,
    articleRetentionDays,
    scriptFeedID,
    scriptContent,
    scriptLang,
    setNetworkProxyURL,
    setAIAPIKey,
    setAIBaseURL,
    setAIModel,
    setAITargetLang,
    setArticleRetentionDays,
    setScriptFeedID,
    setScriptContent,
    setScriptLang,
    setScriptDirty,
    loadFeeds,
    loadFolders,
    loadArticles,
    setMessage,
  });

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
    isTranslatingArticle,
    setIsTranslatingArticle,
    aiTargetLang,
    setTranslationParagraphsByArticleID,
  });

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
    createRootFolder,
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

  useEffect(
    () => () => {
      if (bounceTimerRef.current != null) {
        window.clearTimeout(bounceTimerRef.current);
      }
    },
    [],
  );

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
              onSelectFeed={(feedID) => {
                void selectFeed(feedID);
              }}
              onSelectFolder={(folderID) => {
                void selectFolder(folderID);
              }}
              onSelectArticle={(articleID) => {
                void selectArticle(articleID);
              }}
              onToggleFolderCollapsed={toggleFolderCollapsed}
              onOpenFeedContextMenu={openFeedContextMenu}
              onOpenFolderContextMenu={openFolderContextMenu}
              onFeedDragStart={onFeedDragStart}
              onFeedDragEnd={onFeedDragEnd}
              onFolderDragOver={onFolderDragOver}
              onFolderDragLeave={onFolderDragLeave}
              onFolderDrop={(event, folderID) => {
                void onFolderDrop(event, folderID);
              }}
              onUncategorizedDragOver={onUncategorizedDragOver}
              onUncategorizedDragLeave={onUncategorizedDragLeave}
              onUncategorizedDrop={(event) => {
                void onUncategorizedDrop(event);
              }}
              onRenamingFeedTitleChange={setRenamingFeedTitle}
              onRenameFeed={(feedID) => {
                void renameFeed(feedID);
              }}
            />
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
        onScriptLangChange={(lang) => {
          setScriptLang(lang);
          setScriptDirty(true);
        }}
        onScriptContentChange={(value) => {
          setScriptContent(value);
          setScriptDirty(true);
        }}
        onSaveFeedScript={saveFeedScript}
        apiBase={apiBase}
        onAPIBaseChange={setApiBase}
        onSaveAPIBase={handleSaveAPIBase}
        networkProxyURL={networkProxyURL}
        onNetworkProxyURLChange={setNetworkProxyURL}
        onSaveNetworkSettings={saveNetworkSettings}
        aiAPIKey={aiAPIKey}
        aiBaseURL={aiBaseURL}
        aiModel={aiModel}
        aiTargetLang={aiTargetLang}
        onAIAPIKeyChange={setAIAPIKey}
        onAIBaseURLChange={setAIBaseURL}
        onAIModelChange={setAIModel}
        onAITargetLangChange={setAITargetLang}
        onSaveAISettings={saveAISettings}
        articleRetentionDays={articleRetentionDays}
        onArticleRetentionDaysChange={setArticleRetentionDays}
        onSaveDataSettings={saveDataSettings}
        onExportProfileJSON={exportProfileJSON}
        onExportOPML={exportOPML}
        onImportProfileJSON={importProfileJSON}
        onImportOPML={importOPML}
      />

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
              closeFeedContextMenu();
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
