import type { Folder } from "@/types";

export function buildDescendantFolderIDs(folders: Folder[]): Map<number, Set<number>> {
  const childrenByParent = new Map<number, number[]>();
  folders.forEach((folder) => {
    if (folder.parent_id == null) {
      return;
    }
    const list = childrenByParent.get(folder.parent_id) ?? [];
    list.push(folder.id);
    childrenByParent.set(folder.parent_id, list);
  });

  const cache = new Map<number, Set<number>>();
  const visit = (id: number): Set<number> => {
    const cached = cache.get(id);
    if (cached) {
      return cached;
    }
    const next = new Set<number>([id]);
    (childrenByParent.get(id) ?? []).forEach((childID) => {
      visit(childID).forEach((descendant) => next.add(descendant));
    });
    cache.set(id, next);
    return next;
  };

  folders.forEach((folder) => {
    visit(folder.id);
  });
  return cache;
}
