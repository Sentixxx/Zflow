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
          variant="ghost"
          size="icon"
          onClick={onClick}
          disabled={disabled || loading}
          aria-label={ariaLabel}
          className={cn(
            "h-8 w-8",
            active && "text-primary bg-primary/10",
            loading && "[&_svg]:animate-spin"
          )}
        >
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{title}</TooltipContent>
    </Tooltip>
  );
}
