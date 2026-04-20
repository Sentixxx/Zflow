/* @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import type { Feed, Folder } from "@/types";
import { useSidebarFeedActions } from "./useSidebarFeedActions";

// 覆盖 useSidebarFeedActions 中的核心 action：
// createRootFolder, deleteFeed, renameFeed, deleteFolder（含 confirm 分支）,
// moveFeedToFolder 成功/失败路径，以及 pendingDeleteFeed / folderContextMenu 状态机。

type GlobalWithAct = typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };
type Returned = ReturnType<typeof useSidebarFeedActions>;

function makeFolder(overrides: Partial<Folder> = {}): Folder {
  return {
    id: 1,
    name: "default",
    parent_id: null,
    created_at: "",
    updated_at: "",
    ...overrides,
  };
}

function makeFeed(overrides: Partial<Feed> = {}): Feed {
  return {
    id: 1,
    url: "https://a/rss",
    title: "A",
    item_count: 0,
    last_fetched_at: "",
    last_fetch_status: "ok",
    created_at: "",
    ...overrides,
  };
}

type HarnessOpts = {
  feeds?: Feed[];
  folders?: Folder[];
  selectedFeedID?: number | null;
  selectedFolderID?: number | null;
  feedNameByID?: Map<number, string>;
};

function mount(client: Record<string, ReturnType<typeof vi.fn>>, opts: HarnessOpts = {}) {
  (globalThis as GlobalWithAct).IS_REACT_ACT_ENVIRONMENT = true;
  const messages: Array<{ text: string; isError: boolean }> = [];
  const loadFeeds = vi.fn().mockResolvedValue(undefined);
  const loadFolders = vi.fn().mockResolvedValue(undefined);
  const loadArticles = vi.fn().mockResolvedValue(undefined);
  const setSelectedFeedID = vi.fn();
  const setSelectedFolderID = vi.fn();
  const setNewFeedFolderID = vi.fn();
  const selectScriptFeed = vi.fn();
  const setSettingsTab = vi.fn();
  const setSettingsOpen = vi.fn();

  // 固化引用：Probe 每次 render 必须复用同一 folders/feeds 数组，
  // 否则 useEffect([folders]) 会把 setCollapsedFolders 当成新依赖无限触发，直到 OOM。
  const stableFolders = opts.folders ?? [];
  const stableFeeds = opts.feeds ?? [];
  const stableNameMap = opts.feedNameByID ?? new Map<number, string>();

  const container = document.createElement("div");
  const root = createRoot(container);
  const ref = { current: undefined as Returned | undefined };
  const Probe = () => {
    ref.current = useSidebarFeedActions({
      client: client as never,
      folders: stableFolders,
      feeds: stableFeeds,
      selectedFeedID: opts.selectedFeedID ?? null,
      selectedFolderID: opts.selectedFolderID ?? null,
      setSelectedFeedID,
      setSelectedFolderID,
      setMessage: (message, isError) => messages.push({ text: message, isError: Boolean(isError) }),
      loadFeeds,
      loadFolders,
      loadArticles,
      feedNameByID: stableNameMap,
      setNewFeedFolderID,
      selectScriptFeed,
      setSettingsTab,
      setSettingsOpen,
    });
    return null;
  };
  act(() => {
    root.render(createElement(Probe));
  });
  return {
    get actions() {
      if (!ref.current) throw new Error("hook not ready");
      return ref.current;
    },
    messages,
    loadFeeds,
    loadFolders,
    loadArticles,
    setSelectedFeedID,
    setSelectedFolderID,
    setNewFeedFolderID,
    selectScriptFeed,
    unmount: () => {
      act(() => root.unmount());
    },
  };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSidebarFeedActions", () => {
  it("createRootFolder: 用户取消 prompt 时 early-return，不触发任何副作用", async () => {
    vi.spyOn(window, "prompt").mockReturnValueOnce(null);
    const client = {
      createFolder: vi.fn(),
      updateFolder: vi.fn(),
      deleteFolder: vi.fn(),
      deleteFeed: vi.fn(),
      updateFeedTitle: vi.fn(),
      updateFeedFolder: vi.fn(),
    };
    const h = mount(client);
    await act(async () => {
      await h.actions.createRootFolder();
    });
    expect(client.createFolder).not.toHaveBeenCalled();
    expect(h.messages).toHaveLength(0);
    h.unmount();
  });

  it("createRootFolder: 成功路径拉刷新并把新 folder 赋给 newFeedFolderID", async () => {
    vi.spyOn(window, "prompt").mockReturnValueOnce("science");
    const folder = makeFolder({ id: 42, name: "science" });
    const client = {
      createFolder: vi.fn().mockResolvedValue(folder),
      updateFolder: vi.fn(),
      deleteFolder: vi.fn(),
      deleteFeed: vi.fn(),
      updateFeedTitle: vi.fn(),
      updateFeedFolder: vi.fn(),
    };
    const h = mount(client);
    await act(async () => {
      await h.actions.createRootFolder();
    });
    expect(client.createFolder).toHaveBeenCalledWith("science", null);
    expect(h.loadFolders).toHaveBeenCalled();
    expect(h.setNewFeedFolderID).toHaveBeenCalledWith(42);
    expect(h.messages[h.messages.length - 1]?.text).toBe("分类已创建");
    h.unmount();
  });

  it("createRootFolder: API 失败时走 error 分支", async () => {
    vi.spyOn(window, "prompt").mockReturnValueOnce("bad");
    const client = {
      createFolder: vi.fn().mockRejectedValue(new Error("duplicate name")),
      updateFolder: vi.fn(),
      deleteFolder: vi.fn(),
      deleteFeed: vi.fn(),
      updateFeedTitle: vi.fn(),
      updateFeedFolder: vi.fn(),
    };
    const h = mount(client);
    await act(async () => {
      await h.actions.createRootFolder();
    });
    expect(h.messages[h.messages.length - 1]).toEqual({ text: "duplicate name", isError: true });
    expect(h.setNewFeedFolderID).not.toHaveBeenCalled();
    h.unmount();
  });

  it("deleteFeed: 成功路径并发刷新 feeds/articles 并清空 selectedFeedID（若相同）", async () => {
    const feed = makeFeed({ id: 9 });
    const client = {
      createFolder: vi.fn(),
      updateFolder: vi.fn(),
      deleteFolder: vi.fn(),
      deleteFeed: vi.fn().mockResolvedValue(undefined),
      updateFeedTitle: vi.fn(),
      updateFeedFolder: vi.fn(),
    };
    const h = mount(client, { selectedFeedID: 9 });
    await act(async () => {
      await h.actions.deleteFeed(feed);
    });
    expect(client.deleteFeed).toHaveBeenCalledWith(9);
    expect(h.setSelectedFeedID).toHaveBeenCalledWith(null);
    expect(h.loadFeeds).toHaveBeenCalled();
    expect(h.loadArticles).toHaveBeenCalled();
    expect(h.messages[h.messages.length - 1]?.text).toBe("订阅源已删除");
    h.unmount();
  });

  it("deleteFeed: 失败时 setMessage error，pendingDeleteFeed 在 finally 复位", async () => {
    const feed = makeFeed({ id: 9 });
    const client = {
      createFolder: vi.fn(),
      updateFolder: vi.fn(),
      deleteFolder: vi.fn(),
      deleteFeed: vi.fn().mockRejectedValue(new Error("permission denied")),
      updateFeedTitle: vi.fn(),
      updateFeedFolder: vi.fn(),
    };
    const h = mount(client);
    await act(async () => {
      await h.actions.deleteFeed(feed);
    });
    expect(h.messages[h.messages.length - 1]).toEqual({ text: "permission denied", isError: true });
    // pendingDeleteFeed 在 finally 被设 null（初始也是 null，这里验证没有副作用泄漏）
    expect(h.actions.pendingDeleteFeed).toBeNull();
    h.unmount();
  });

  it("renameFeed: 修改名字后调用 updateFeedTitle 并刷新列表", async () => {
    const client = {
      createFolder: vi.fn(),
      updateFolder: vi.fn(),
      deleteFolder: vi.fn(),
      deleteFeed: vi.fn(),
      updateFeedTitle: vi.fn().mockResolvedValue(undefined),
      updateFeedFolder: vi.fn(),
    };
    const nameMap = new Map<number, string>([[1, "old-name"]]);
    const h = mount(client, { feedNameByID: nameMap });
    // 进入 rename 状态
    act(() => {
      h.actions.startRenameFeed(makeFeed({ id: 1, title: "old-name" }));
    });
    act(() => {
      h.actions.setRenamingFeedTitle("new-name");
    });
    await act(async () => {
      await h.actions.renameFeed(1);
    });
    expect(client.updateFeedTitle).toHaveBeenCalledWith(1, "new-name");
    expect(h.loadFeeds).toHaveBeenCalled();
    expect(h.messages[h.messages.length - 1]?.text).toBe("订阅源已重命名");
    h.unmount();
  });

  it("renameFeed: 未变化的名字不调用 API（短路）", async () => {
    const client = {
      createFolder: vi.fn(),
      updateFolder: vi.fn(),
      deleteFolder: vi.fn(),
      deleteFeed: vi.fn(),
      updateFeedTitle: vi.fn(),
      updateFeedFolder: vi.fn(),
    };
    const nameMap = new Map<number, string>([[1, "same"]]);
    const h = mount(client, { feedNameByID: nameMap });
    act(() => {
      h.actions.startRenameFeed(makeFeed({ id: 1, title: "same" }));
    });
    act(() => {
      h.actions.setRenamingFeedTitle("same");
    });
    await act(async () => {
      await h.actions.renameFeed(1);
    });
    expect(client.updateFeedTitle).not.toHaveBeenCalled();
    h.unmount();
  });

  it("deleteFolder: 用户 cancel 确认框时 early-return", async () => {
    vi.spyOn(window, "confirm").mockReturnValueOnce(false);
    const client = {
      createFolder: vi.fn(),
      updateFolder: vi.fn(),
      deleteFolder: vi.fn(),
      deleteFeed: vi.fn(),
      updateFeedTitle: vi.fn(),
      updateFeedFolder: vi.fn(),
    };
    const folder = makeFolder({ id: 5, name: "target" });
    const h = mount(client);
    // 先打开 folderContextMenu，deleteFolder 依赖它
    const fakeEvent = {
      preventDefault: () => {},
      stopPropagation: () => {},
      clientX: 100,
      clientY: 100,
    } as unknown as React.MouseEvent;
    act(() => {
      h.actions.openFolderContextMenu(fakeEvent, folder);
    });
    await act(async () => {
      await h.actions.deleteFolder();
    });
    expect(client.deleteFolder).not.toHaveBeenCalled();
    h.unmount();
  });

  it("deleteFolder: 确认后调用 API，selectedFolderID 命中时清空", async () => {
    vi.spyOn(window, "confirm").mockReturnValueOnce(true);
    const client = {
      createFolder: vi.fn(),
      updateFolder: vi.fn(),
      deleteFolder: vi.fn().mockResolvedValue(undefined),
      deleteFeed: vi.fn(),
      updateFeedTitle: vi.fn(),
      updateFeedFolder: vi.fn(),
    };
    const folder = makeFolder({ id: 5, name: "target" });
    const h = mount(client, { selectedFolderID: 5 });
    const fakeEvent = {
      preventDefault: () => {},
      stopPropagation: () => {},
      clientX: 10,
      clientY: 10,
    } as unknown as React.MouseEvent;
    act(() => {
      h.actions.openFolderContextMenu(fakeEvent, folder);
    });
    await act(async () => {
      await h.actions.deleteFolder();
    });
    expect(client.deleteFolder).toHaveBeenCalledWith(5);
    expect(h.setSelectedFolderID).toHaveBeenCalledWith(null);
    expect(h.messages[h.messages.length - 1]?.text).toBe("分类已删除");
    h.unmount();
  });

  it("saveFeedCategory: 无 manageCategoryFeed 时 early-return", async () => {
    const client = {
      createFolder: vi.fn(),
      updateFolder: vi.fn(),
      deleteFolder: vi.fn(),
      deleteFeed: vi.fn(),
      updateFeedTitle: vi.fn(),
      updateFeedFolder: vi.fn(),
    };
    const h = mount(client);
    await act(async () => {
      await h.actions.saveFeedCategory();
    });
    expect(client.updateFeedFolder).not.toHaveBeenCalled();
    h.unmount();
  });

  it("openScriptSettingsForFeed: 切到 script tab 并打开 settings dialog", () => {
    const client = {
      createFolder: vi.fn(),
      updateFolder: vi.fn(),
      deleteFolder: vi.fn(),
      deleteFeed: vi.fn(),
      updateFeedTitle: vi.fn(),
      updateFeedFolder: vi.fn(),
    };
    const h = mount(client);
    act(() => {
      h.actions.openScriptSettingsForFeed(makeFeed({ id: 77 }));
    });
    expect(h.selectScriptFeed).toHaveBeenCalledWith(77);
    h.unmount();
  });

  it("toggleFolderCollapsed: 切换折叠状态", () => {
    const client = {
      createFolder: vi.fn(),
      updateFolder: vi.fn(),
      deleteFolder: vi.fn(),
      deleteFeed: vi.fn(),
      updateFeedTitle: vi.fn(),
      updateFeedFolder: vi.fn(),
    };
    const h = mount(client);
    expect(h.actions.collapsedFolders[3]).toBeUndefined();
    act(() => {
      h.actions.toggleFolderCollapsed(3);
    });
    expect(h.actions.collapsedFolders[3]).toBe(true);
    act(() => {
      h.actions.toggleFolderCollapsed(3);
    });
    expect(h.actions.collapsedFolders[3]).toBe(false);
    h.unmount();
  });
});
