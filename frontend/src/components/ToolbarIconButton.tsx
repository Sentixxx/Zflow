import type { ReactNode } from "react";

type ToolbarIconButtonProps = {
  title: string;
  ariaLabel: string;
  disabled?: boolean;
  onClick: () => void;
  children: ReactNode;
};

export function ToolbarIconButton({ title, ariaLabel, disabled = false, onClick, children }: ToolbarIconButtonProps) {
  return (
    <button className="toolbar-icon-btn" onClick={onClick} disabled={disabled} title={title} aria-label={ariaLabel}>
      {children}
    </button>
  );
}
