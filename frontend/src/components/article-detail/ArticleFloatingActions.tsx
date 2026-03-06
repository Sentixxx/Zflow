type FloatingAction = {
  id: string;
  label: string;
  icon: string;
  title: string;
};

const FLOATING_ACTIONS: FloatingAction[] = [
  { id: "prev", label: "上一篇", icon: "←", title: "上一篇" },
  { id: "next", label: "下一篇", icon: "→", title: "下一篇" },
  { id: "scroll-top", label: "回到顶部", icon: "↑", title: "回到顶部" },
  { id: "translate", label: "一键翻译", icon: "译", title: "一键翻译" },
];

type ArticleFloatingActionsProps = {
  onPrev: () => void;
  onNext: () => void;
  canGoPrev: boolean;
  canGoNext: boolean;
  onScrollTop: () => void;
  onTranslate: () => void;
  isTranslating: boolean;
};

export function ArticleFloatingActions({ onPrev, onNext, canGoPrev, canGoNext, onScrollTop, onTranslate, isTranslating }: ArticleFloatingActionsProps) {
  return (
    <div className="article-floating-proximity" aria-hidden="false">
      <aside className="article-floating-actions" aria-label="文章快捷操作">
        {FLOATING_ACTIONS.map((action) => (
          <button
            key={action.id}
            type="button"
            className="floating-action-btn"
            title={action.title}
            aria-label={action.label}
            disabled={(action.id === "translate" && isTranslating) || (action.id === "prev" && !canGoPrev) || (action.id === "next" && !canGoNext)}
            onClick={() => {
              if (action.id === "prev") {
                onPrev();
                return;
              }
              if (action.id === "next") {
                onNext();
                return;
              }
              if (action.id === "scroll-top") {
                onScrollTop();
                return;
              }
              if (action.id === "translate") {
                onTranslate();
              }
            }}
          >
            <span className="floating-action-icon" aria-hidden="true">
              {action.icon}
            </span>
          </button>
        ))}
      </aside>
    </div>
  );
}
