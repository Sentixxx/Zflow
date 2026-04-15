/**
 * useReaderStore contract tests.
 *
 * Invariants:
 * - Default state: selectedFeedID=null, selectedFolderID=null,
 *   readFilter="all", sortMode="latest"
 * - Setters are independent: setting feedID does not reset folderID and vice versa.
 * - State updates are reflected synchronously after the setter call.
 */
import { beforeEach, describe, expect, it } from "vitest";
import { useReaderStore } from "./useReaderStore";

// Reset store state before each test to prevent inter-test leakage.
beforeEach(() => {
  useReaderStore.setState({
    selectedFeedID: null,
    selectedFolderID: null,
    readFilter: "all",
    sortMode: "latest",
  });
});

describe("useReaderStore defaults", () => {
  it("initializes with correct default values", () => {
    const state = useReaderStore.getState();
    expect(state.selectedFeedID).toBeNull();
    expect(state.selectedFolderID).toBeNull();
    expect(state.readFilter).toBe("all");
    expect(state.sortMode).toBe("latest");
  });
});

describe("useReaderStore setSelectedFeedID", () => {
  it("updates selectedFeedID without clearing selectedFolderID", () => {
    useReaderStore.getState().setSelectedFolderID(5);
    useReaderStore.getState().setSelectedFeedID(10);
    const state = useReaderStore.getState();
    expect(state.selectedFeedID).toBe(10);
    // folderID must remain untouched — the two selectors are independent
    expect(state.selectedFolderID).toBe(5);
  });

  it("can be reset to null", () => {
    useReaderStore.getState().setSelectedFeedID(42);
    useReaderStore.getState().setSelectedFeedID(null);
    expect(useReaderStore.getState().selectedFeedID).toBeNull();
  });
});

describe("useReaderStore setSelectedFolderID", () => {
  it("updates selectedFolderID without clearing selectedFeedID", () => {
    useReaderStore.getState().setSelectedFeedID(3);
    useReaderStore.getState().setSelectedFolderID(7);
    const state = useReaderStore.getState();
    expect(state.selectedFolderID).toBe(7);
    // feedID must remain untouched
    expect(state.selectedFeedID).toBe(3);
  });
});

describe("useReaderStore setReadFilter", () => {
  it("switches from all to unread", () => {
    useReaderStore.getState().setReadFilter("unread");
    expect(useReaderStore.getState().readFilter).toBe("unread");
  });

  it("switches back to all", () => {
    useReaderStore.getState().setReadFilter("unread");
    useReaderStore.getState().setReadFilter("all");
    expect(useReaderStore.getState().readFilter).toBe("all");
  });
});

describe("useReaderStore setSortMode", () => {
  it("updates sortMode to each supported value", () => {
    const modes = ["latest", "oldest", "recommend", "quality", "relevance", "novelty"] as const;
    for (const mode of modes) {
      useReaderStore.getState().setSortMode(mode);
      expect(useReaderStore.getState().sortMode).toBe(mode);
    }
  });
});
