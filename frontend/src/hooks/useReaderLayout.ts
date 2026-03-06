import { useEffect, useMemo, useState } from "react";
import type { MouseEvent as ReactMouseEvent } from "react";

const MIN_SIDEBAR_WIDTH = 260;
const MAX_SIDEBAR_WIDTH = 520;
const MIN_LIST_WIDTH = 300;
const MAX_LIST_WIDTH = 620;
const COLLAPSED_SIDEBAR_WIDTH = 56;
const RESIZER_WIDTH = 8;

type ResizeTarget = "sidebar" | "list";

type Params = {
  sidebarCollapsed: boolean;
};

export function useReaderLayout({ sidebarCollapsed }: Params) {
  const [sidebarWidth, setSidebarWidth] = useState<number>(360);
  const [listWidth, setListWidth] = useState<number>(360);
  const [isNarrow, setIsNarrow] = useState<boolean>(() => window.innerWidth <= 900);

  useEffect(() => {
    const onResize = () => {
      setIsNarrow(window.innerWidth <= 900);
    };
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  const layoutStyle = useMemo(() => {
    const computedSidebarWidth = sidebarCollapsed ? COLLAPSED_SIDEBAR_WIDTH : sidebarWidth;
    return {
      gridTemplateColumns: isNarrow
        ? "1fr"
        : `${computedSidebarWidth}px ${RESIZER_WIDTH}px ${listWidth}px ${RESIZER_WIDTH}px minmax(0, 1fr)`,
    };
  }, [isNarrow, listWidth, sidebarCollapsed, sidebarWidth]);

  const beginResize = (target: ResizeTarget) => (event: ReactMouseEvent<HTMLDivElement>) => {
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

  return {
    isNarrow,
    layoutStyle,
    beginResize,
  };
}
