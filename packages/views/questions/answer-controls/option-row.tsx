"use client";

import type { ReactNode } from "react";
import { Check } from "lucide-react";
import { Kbd } from "@multica/ui/components/ui/kbd";
import { cn } from "@multica/ui/lib/utils";
import { SuggestedBadge } from "./suggested-badge";

/**
 * One option of Single or Multiple choice: the library's `Option`, with a
 * radio or checkbox mark, the Suggested badge and its number key.
 */
export function OptionRow({
  mark,
  selected,
  suggested,
  shortcut,
  disabled,
  onSelect,
  children,
}: {
  mark: "radio" | "checkbox";
  selected: boolean;
  suggested?: boolean;
  shortcut?: number;
  disabled: boolean;
  onSelect: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      role={mark}
      aria-checked={selected}
      disabled={disabled}
      onClick={onSelect}
      className={cn(
        "flex min-h-10 w-full items-center gap-3 rounded-lg border px-3 py-2 text-left text-body transition-colors outline-none",
        "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50",
        selected ? "border-foreground bg-muted" : "border-border hover:bg-muted/50",
      )}
    >
      {mark === "radio" ? (
        <span
          aria-hidden
          className={cn(
            "flex size-4 shrink-0 items-center justify-center rounded-full border",
            selected ? "border-foreground" : "border-input",
          )}
        >
          {selected && <span className="size-2 rounded-full bg-foreground" />}
        </span>
      ) : (
        <span
          aria-hidden
          className={cn(
            "flex size-4 shrink-0 items-center justify-center rounded-xs border",
            selected ? "border-primary bg-primary text-primary-foreground" : "border-input",
          )}
        >
          {selected && <Check className="size-3" />}
        </span>
      )}
      <span className="min-w-0 flex-1 break-words">{children}</span>
      {suggested && <SuggestedBadge />}
      {shortcut !== undefined && <Kbd className="border bg-transparent">{shortcut}</Kbd>}
    </button>
  );
}
