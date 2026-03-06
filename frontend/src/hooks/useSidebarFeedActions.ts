import { useEffect, useState } from "react";
import type { Feed, Folder } from "@/types";
import type { SettingsTab } from "@/components";

export type FolderContextMenu = { folder: Folder; x: number; y: number } | null;
export type FeedContextMenu = { feed: Feed; x: number; y: number } | null;

type Params = {
  client: {
    createFolder: (name: string, parentID?: number | null) => Promise<Folder>;
    updateFolder: (id: number, name: string, parentID: number | null) => Promise<unknown>;
    deleteFolder: (id: number) => Promise<unknown>;
    deleteFeed: (id: number) => Promise<unknown>;
    updateFeedTitle: (id: number, title: string) => Promise<unknown>;
    updateFeedFolder: (feedID: number, folderID: number | null) => Promise<unknown>;
  };
  folders: Folder[];
  feeds: Feed[];
  selectedFeedID: number | null;
  selectedFolderID: number | null;
  setSelectedFeedID: (id: number | null) => void;
  setSelectedFolderID: (id: number | null) => void;
  setMessage: (message: string, isError?: boolean) => void;
  loadFeeds: (opts?: { silentStatus?: boolean }) => Promise<unknown>;
  loadFolders: (opts?: { silentStatus?: boolean }) => Promise<unknown>;
  loadArticles: (opts?: { silentStatus?: boolean }) => Promise<unknown>;
  feedNameByID: Map<number, string>;
  setNewFeedFolderID: (id: number | null) => void;
  selectScriptFeed: (feedID: number | null) => void;
  setSettingsTab: (tab: SettingsTab) => void;
  setSettingsOpen: (open: boolean) => void;
};

export function useSidebarFeedActions({
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
}: Params) {
  const [folderContextMenu, setFolderContextMenu] = useState<FolderContextMenu>(null);
  const [feedContextMenu, setFeedContextMenu] = useState<FeedContextMenu>(null);
  const [renamingFeedID, setRenamingFeedID] = useState<number | null>(null);
  const [renamingFeedTitle, setRenamingFeedTitle] = useState<string>("");
  const [renamingFeedOriginalTitle, setRenamingFeedOriginalTitle] = useState<string>("");
  const [manageCategoryFeed, setManageCategoryFeed] = useState<Feed | null>(null);
  const [manageCategoryFolderID, setManageCategoryFolderID] = useState<number | null>(null);
  const [collapsedFolders, setCollapsedFolders] = useState<Record<number, boolean>>({});
  const [draggingFeedID, setDraggingFeedID] = useState<number | null>(null);
  const [dragOverFolderID, setDragOverFolderID] = useState<number | null>(null);
  const [dragOverUncategorized, setDragOverUncategorized] = useState<boolean>(false);
  const [dragOverDeleteZone, setDragOverDeleteZone] = useState<boolean>(false);
  const [pendingDeleteFeed, setPendingDeleteFeed] = useState<Feed | null>(null);

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

  const openFolderContextMenu = (event: React.MouseEvent, folder: Folder) => {
    event.preventDefault();
    event.stopPropagation();
    const menuWidth = 188;
    const menuHeight = 170;
    const x = Math.max(10, Math.min(event.clientX, window.innerWidth - menuWidth - 10));
    const y = Math.max(10, Math.min(event.clientY, window.innerHeight - menuHeight - 10));
    setFolderContextMenu({ folder, x, y });
    setFeedContextMenu(null);
  };

  const openFeedContextMenu = (event: React.MouseEvent, feed: Feed) => {
    event.preventDefault();
    event.stopPropagation();
    const menuWidth = 196;
    const menuHeight = 206;
    const x = Math.max(10, Math.min(event.clientX, window.innerWidth - menuWidth - 10));
    const y = Math.max(10, Math.min(event.clientY, window.innerHeight - menuHeight - 10));
    setFeedContextMenu({ feed, x, y });
    setFolderContextMenu(null);
  };

  const closeFeedContextMenu = () => {
    setFeedContextMenu(null);
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

  const toggleFolderCollapsed = (folderID: number) => {
    setCollapsedFolders((current) => ({ ...current, [folderID]: !current[folderID] }));
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

  const onFolderDragLeave = (folderID: number) => {
    setDragOverFolderID((current) => (current === folderID ? null : current));
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

  const onUncategorizedDragLeave = () => {
    setDragOverUncategorized(false);
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

  return {
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
  };
}
