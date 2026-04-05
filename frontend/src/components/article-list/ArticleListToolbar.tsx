import { SORT_MODE_LABELS } from "@/lib/article-list";
import type { ReadFilter, SortMode } from "@/lib/article-list";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { ChevronDown } from "lucide-react";

type ArticleListToolbarProps = {
  readFilter: ReadFilter;
  sortMode: SortMode;
  onToggleReadFilter: () => void;
  onSortModeChange: (mode: SortMode) => void;
};

const SORT_MODE_OPTIONS: SortMode[] = ["latest", "oldest", "recommend", "quality", "relevance", "novelty"];

export function ArticleListToolbar({ readFilter, sortMode, onToggleReadFilter, onSortModeChange }: ArticleListToolbarProps) {
  return (
    <div className="flex items-center gap-2">
      {/* Read filter toggle */}
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="outline"
            size="icon"
            onClick={onToggleReadFilter}
            aria-label={readFilter === "unread" ? "仅显示未读" : "显示全部（含已读）"}
            className={cn(readFilter === "unread" && "bg-accent text-accent-foreground border-primary/40")}
          >
            <span className="text-base leading-none">{readFilter === "unread" ? "○" : "●"}</span>
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">
          {readFilter === "unread" ? "仅显示未读" : "显示全部（含已读）"}
        </TooltipContent>
      </Tooltip>

      {/* Sort dropdown */}
      <div className="flex items-center gap-1.5">
        <span className="text-xs font-bold uppercase tracking-widest text-muted-foreground select-none">排序</span>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="outline"
              className="h-control-md gap-2 px-3 rounded-full text-base font-semibold min-w-[120px] justify-between"
              aria-label={`文章排序方式：${SORT_MODE_LABELS[sortMode]}`}
            >
              <span>{SORT_MODE_LABELS[sortMode]}</span>
              <ChevronDown className="h-3.5 w-3.5 opacity-50 shrink-0" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="min-w-[150px]">
            <DropdownMenuRadioGroup value={sortMode} onValueChange={(v) => onSortModeChange(v as SortMode)}>
              {SORT_MODE_OPTIONS.map((mode) => (
                <DropdownMenuRadioItem key={mode} value={mode}>
                  {SORT_MODE_LABELS[mode]}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}
