import { create } from "zustand";
import type { ReadFilter, SortMode } from "@/lib/article-list";

type ReaderStore = {
  selectedFeedID: number | null;
  selectedFolderID: number | null;
  readFilter: ReadFilter;
  sortMode: SortMode;
  setSelectedFeedID: (id: number | null) => void;
  setSelectedFolderID: (id: number | null) => void;
  setReadFilter: (filter: ReadFilter) => void;
  setSortMode: (mode: SortMode) => void;
};

export const useReaderStore = create<ReaderStore>((set) => ({
  selectedFeedID: null,
  selectedFolderID: null,
  readFilter: "all",
  sortMode: "latest",
  setSelectedFeedID: (selectedFeedID) => set({ selectedFeedID }),
  setSelectedFolderID: (selectedFolderID) => set({ selectedFolderID }),
  setReadFilter: (readFilter) => set({ readFilter }),
  setSortMode: (sortMode) => set({ sortMode }),
}));
