import { ChevronLeft, ChevronRight, ArrowUp, Languages } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type ArticleFloatingActionsProps = {
  onPrev: () => void;
  onNext: () => void;
  canGoPrev: boolean;
  canGoNext: boolean;
  onScrollTop: () => void;
  onTranslate: () => void;
  isTranslating: boolean;
};

export function ArticleFloatingActions({
  onPrev,
  onNext,
  canGoPrev,
  canGoNext,
  onScrollTop,
  onTranslate,
  isTranslating,
}: ArticleFloatingActionsProps) {
  const actions = [
    { id: "prev", icon: <ChevronLeft className="w-4 h-4" />, label: "上一篇", disabled: !canGoPrev, onClick: onPrev },
    { id: "next", icon: <ChevronRight className="w-4 h-4" />, label: "下一篇", disabled: !canGoNext, onClick: onNext },
    { id: "scroll-top", icon: <ArrowUp className="w-4 h-4" />, label: "回到顶部", disabled: false, onClick: onScrollTop },
    {
      id: "translate",
      icon: <Languages className={cn("w-4 h-4", isTranslating && "animate-pulse")} />,
      label: isTranslating ? "翻译中..." : "一键翻译",
      disabled: isTranslating,
      onClick: onTranslate,
    },
  ] as const;

  return (
    <aside
      className="fixed bottom-6 right-6 flex flex-col gap-1.5 z-30"
      aria-label="文章快捷操作"
    >
      {actions.map((action) => (
        <Tooltip key={action.id}>
          <TooltipTrigger asChild>
            <Button
              variant="secondary"
              size="icon"
              disabled={action.disabled}
              onClick={action.onClick}
              aria-label={action.label}
              className="rounded-full shadow-md border border-border/60 w-9 h-9"
            >
              {action.icon}
            </Button>
          </TooltipTrigger>
          <TooltipContent side="left">{action.label}</TooltipContent>
        </Tooltip>
      ))}
    </aside>
  );
}
