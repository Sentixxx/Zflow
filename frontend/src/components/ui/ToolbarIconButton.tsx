import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type ToolbarIconButtonProps = {
  title: string;
  ariaLabel: string;
  disabled?: boolean;
  loading?: boolean;
  active?: boolean;
  onClick: () => void;
  children: ReactNode;
};

export function ToolbarIconButton({
  title,
  ariaLabel,
  disabled = false,
  loading = false,
  active = false,
  onClick,
  children,
}: ToolbarIconButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="outline"
          size="icon"
          onClick={onClick}
          disabled={disabled || loading}
          aria-label={ariaLabel}
          className={cn(active && "bg-accent text-accent-foreground border-primary/40")}
        >
          <span
            className={cn("text-base font-semibold leading-none", loading && "animate-[spin_0.78s_linear_infinite]")}
            style={{ display: "inline-block" }}
          >
            {children}
          </span>
        </Button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{title}</TooltipContent>
    </Tooltip>
  );
}
