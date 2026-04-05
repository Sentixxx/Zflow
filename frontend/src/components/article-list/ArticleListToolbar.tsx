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
import { ChevronDown, Eye, EyeOff } from "lucide-react";

type ArticleListToolbarProps = {
  readFilter: ReadFilter;
  sortMode: SortMode;
  onToggleReadFilter: () => void;
  onSortModeChange: (mode: SortMode) => void;
};

const SORT_MODE_OPTIONS: SortMode[] = ["latest", "oldest", "recommend", "quality", "relevance", "novelty"];

export function ArticleListToolbar({ readFilter, sortMode, onToggleReadFilter, onSortModeChange }: ArticleListToolbarProps) {
  const isUnreadOnly = readFilter === "unread";

  return (
    <div className="flex items-center gap-1.5">
      {/* Read filter toggle */}
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            onClick={onToggleReadFilter}
            aria-label={isUnreadOnly ? "仅显示未读" : "显示全部（含已读）"}
            className={cn(
              "h-8 w-8",
              isUnreadOnly && "text-primary"
            )}
          >
            {isUnreadOnly
              ? <EyeOff className="w-4 h-4" />
              : <Eye className="w-4 h-4" />
            }
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">
          {isUnreadOnly ? "仅显示未读，点击切换为全部" : "显示全部，点击切换为仅未读"}
        </TooltipContent>
      </Tooltip>

      {/* Sort dropdown */}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className="h-8 gap-1.5 px-2 text-sm font-medium text-muted-foreground hover:text-foreground"
            aria-label={`排序：${SORT_MODE_LABELS[sortMode]}`}
          >
            <span>{SORT_MODE_LABELS[sortMode]}</span>
            <ChevronDown className="h-3.5 w-3.5 opacity-60 shrink-0" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-[140px]">
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
  );
}
