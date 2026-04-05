import {
  BookOpen,
  Star,
  FileText,
  ExternalLink,
  MoreHorizontal,
  RefreshCw,
} from "lucide-react";
import { ToolbarIconButton } from "@/components/ui/ToolbarIconButton";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type ArticleDetailToolbarProps = {
  canMarkUnread: boolean;
  canToggleFavorite: boolean;
  isFavorite: boolean;
  canOpenSourceSite: boolean;
  canExtractReadable: boolean;
  isExtractingReadable: boolean;
  canRefreshArticleCache: boolean;
  isRefreshingArticleCache: boolean;
  hasReadableContent: boolean;
  sourceSiteURL: string;
  onMarkUnread: () => void;
  onToggleFavorite: () => void;
  onOpenSourceSite: () => void;
  onExtractReadable: () => void;
  onRefreshArticleCache: () => void;
};

export function ArticleDetailToolbar({
  canMarkUnread,
  canToggleFavorite,
  isFavorite,
  canOpenSourceSite,
  canExtractReadable,
  isExtractingReadable,
  canRefreshArticleCache,
  isRefreshingArticleCache,
  hasReadableContent,
  sourceSiteURL,
  onMarkUnread,
  onToggleFavorite,
  onOpenSourceSite,
  onExtractReadable,
  onRefreshArticleCache,
}: ArticleDetailToolbarProps) {
  return (
    <div className="flex items-center gap-1 flex-shrink-0">
      <ToolbarIconButton
        onClick={onMarkUnread}
        disabled={!canMarkUnread}
        title={canMarkUnread ? "标记为未读" : "当前已是未读"}
        ariaLabel="标记未读"
      >
        <BookOpen className="w-4 h-4" />
      </ToolbarIconButton>

      <ToolbarIconButton
        onClick={onToggleFavorite}
        disabled={!canToggleFavorite}
        title={isFavorite ? "取消收藏" : "收藏文章"}
        ariaLabel={isFavorite ? "取消收藏" : "收藏文章"}
        active={isFavorite}
      >
        <Star
          className={cn(
            "w-4 h-4 transition-colors",
            isFavorite && "fill-amber-400 text-amber-400"
          )}
        />
      </ToolbarIconButton>

      <ToolbarIconButton
        onClick={onExtractReadable}
        disabled={isExtractingReadable || !canExtractReadable}
        loading={isExtractingReadable}
        title={
          isExtractingReadable
            ? "正在抓取原文..."
            : hasReadableContent
              ? "重新抓取正文"
              : canExtractReadable
                ? "抓取正文"
                : "当前文章缺少可用链接"
        }
        ariaLabel={hasReadableContent ? "重新抓取正文" : "抓取正文"}
      >
        <FileText className="w-4 h-4" />
      </ToolbarIconButton>

      <ToolbarIconButton
        onClick={onOpenSourceSite}
        disabled={!canOpenSourceSite}
        title={canOpenSourceSite ? `在新标签页打开原文：${sourceSiteURL}` : "当前文章缺少可用链接"}
        ariaLabel="打开原文链接"
      >
        <ExternalLink className="w-4 h-4" />
      </ToolbarIconButton>

      {/* More menu */}
      <DropdownMenu>
        <Tooltip>
          <TooltipTrigger asChild>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8" aria-label="更多工具">
                <MoreHorizontal className="w-4 h-4" />
              </Button>
            </DropdownMenuTrigger>
          </TooltipTrigger>
          <TooltipContent side="bottom">更多工具</TooltipContent>
        </Tooltip>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            onClick={onRefreshArticleCache}
            disabled={!canRefreshArticleCache || isRefreshingArticleCache}
          >
            <RefreshCw className={cn("w-3.5 h-3.5 mr-2", isRefreshingArticleCache && "animate-spin")} />
            {isRefreshingArticleCache ? "刷新中..." : "刷新当前文章缓存"}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
