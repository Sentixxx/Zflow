import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type TopBarProps = {
  statusText: string;
  isError: boolean;
  isRefreshingArticles: boolean;
  isRefreshingFeeds: boolean;
  onRefreshArticles: () => void;
  onRefreshFeeds: () => void;
};

export function TopBar({
  statusText,
  isError,
  isRefreshingArticles,
  isRefreshingFeeds,
  onRefreshArticles,
  onRefreshFeeds,
}: TopBarProps) {
  return (
    <header
      className="flex items-center justify-between relative px-4 rounded-xl border border-border shadow-sm"
      style={{ height: "var(--topbar-height)", background: "hsl(var(--background) / 0.92)", backdropFilter: "blur(6px)" }}
    >
      {/* Brand */}
      <div className="flex items-center gap-2.5">
        <span className="text-lg font-bold text-amber-500 dark:text-amber-400">▸</span>
        <h1 className="text-xl font-semibold tracking-tight m-0">Zflow</h1>
      </div>

      {/* Center status */}
      <div
        className={cn(
          "absolute left-1/2 -translate-x-1/2 max-w-[48vw] truncate text-sm text-muted-foreground pointer-events-none",
          isError && "text-destructive"
        )}
      >
        {statusText}
      </div>

      {/* Actions */}
      <div className="flex gap-2">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="outline"
              size="icon"
              onClick={onRefreshArticles}
              disabled={isRefreshingArticles || isRefreshingFeeds}
              aria-label={isRefreshingArticles ? "正在刷新文章" : "刷新文章列表"}
            >
              <span
                className={cn("text-lg font-semibold leading-none", isRefreshingArticles && "animate-[spin_0.78s_linear_infinite]")}
                style={{ display: "inline-block" }}
              >
                ⟳
              </span>
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">
            {isRefreshingArticles ? "正在刷新文章..." : "刷新文章列表"}
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="outline"
              size="icon"
              onClick={onRefreshFeeds}
              disabled={isRefreshingFeeds || isRefreshingArticles}
              aria-label={isRefreshingFeeds ? "正在远端抓取订阅源" : "远端抓取订阅源"}
            >
              <span
                className={cn("text-lg font-semibold leading-none", isRefreshingFeeds && "animate-[spin_0.78s_linear_infinite]")}
                style={{ display: "inline-block" }}
              >
                ◎
              </span>
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">
            {isRefreshingFeeds ? "正在远端抓取订阅源..." : "远端抓取订阅源"}
          </TooltipContent>
        </Tooltip>
      </div>
    </header>
  );
}
