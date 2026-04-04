import { describe, expect, it } from "vitest";
import { buildDescendantFolderIDs } from "./folder-tree";
import type { Folder } from "@/types";

const folders: Folder[] = [
  { id: 1, name: "root", parent_id: null },
  { id: 2, name: "child", parent_id: 1 },
  { id: 3, name: "grand", parent_id: 2 },
];

describe("buildDescendantFolderIDs", () => {
  it("builds descendant set including self", () => {
    const map = buildDescendantFolderIDs(folders);
    expect(Array.from(map.get(1) ?? [])).toEqual([1, 2, 3]);
    expect(Array.from(map.get(2) ?? [])).toEqual([2, 3]);
  });
});
