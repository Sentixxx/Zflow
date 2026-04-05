import { Button } from "@/components/ui/button";

export type RefreshFailure = {
  feedID: number;
  feedTitle: string;
  reason: string;
};

type RefreshFailureBannerProps = {
  failures: RefreshFailure[];
  onClose: () => void;
};

export function RefreshFailureBanner({ failures, onClose }: RefreshFailureBannerProps) {
  if (failures.length === 0) {
    return null;
  }

  return (
    <section
      className="border-b border-destructive/30 bg-destructive/5 px-4 py-2.5"
      role="status"
      aria-live="polite"
    >
      <div className="flex items-center justify-between gap-3 mb-1.5">
        <strong className="text-sm text-destructive font-medium">
          以下订阅源远端抓取失败（{failures.length}）
        </strong>
        <Button variant="ghost" size="sm" onClick={onClose} className="h-6 px-2 text-xs">
          关闭
        </Button>
      </div>
      <ul className="space-y-0.5">
        {failures.slice(0, 6).map((item) => (
          <li key={`${item.feedID}-${item.feedTitle}`} className="flex items-center gap-2 text-xs">
            <span className="font-medium text-foreground truncate max-w-[180px]">{item.feedTitle}</span>
            <span className="text-muted-foreground truncate">{item.reason}</span>
          </li>
        ))}
        {failures.length > 6 && (
          <li className="text-xs text-muted-foreground italic">
            还有 {failures.length - 6} 个失败源，请查看后端日志
          </li>
        )}
      </ul>
    </section>
  );
}
