type FloatingAction = {
  id: string;
  label: string;
  icon: string;
  title: string;
};

const FLOATING_ACTIONS: FloatingAction[] = [
  { id: "scroll-top", label: "回到顶部", icon: "↑", title: "回到顶部（即将支持）" },
  { id: "translate", label: "一键翻译", icon: "译", title: "一键翻译（即将支持）" },
];

export function ArticleFloatingActions() {
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
            onClick={() => {
              // UI-only scaffold: keep future action hooks stable.
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
