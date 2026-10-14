"use client";

import { useRef } from "react";
import type { IssueTemplate, IssueType } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { TypeIcon } from "../type-icon";
import { useT } from "../../../i18n";

interface TemplateListProps {
  templates: readonly IssueTemplate[];
  /** The template picked, or null for No template. */
  type: IssueType | null;
  onPick: (type: IssueType | null) => void;
  /** Blocks picking, while the No template description still uploads a file. */
  disabled?: boolean;
  className?: string;
}

/**
 * New issue's templates as a radio group: No template first, then each
 * template with its type icon, its name and its hint. Arrow keys move the
 * pick, as in a native radio group.
 */
export function TemplateList({ templates, type, onPick, disabled, className }: TemplateListProps) {
  const { t } = useT("issues");
  const listRef = useRef<HTMLDivElement>(null);
  const options: (IssueType | null)[] = [null, ...templates.map((template) => template.type)];

  const move = (step: number) => {
    if (disabled) return;
    const index = options.indexOf(type);
    const next = options[(index + step + options.length) % options.length];
    if (next === undefined) return;
    onPick(next);
    listRef.current
      ?.querySelector<HTMLButtonElement>(`[data-template="${next ?? "none"}"]`)
      ?.focus();
  };

  return (
    <div
      ref={listRef}
      role="radiogroup"
      aria-label={t(($) => $.templates.label)}
      aria-disabled={disabled || undefined}
      className={cn("flex flex-col gap-0.5", className)}
      onKeyDown={(e) => {
        if (e.key === "ArrowDown" || e.key === "ArrowRight") {
          e.preventDefault();
          move(1);
        } else if (e.key === "ArrowUp" || e.key === "ArrowLeft") {
          e.preventDefault();
          move(-1);
        }
      }}
    >
      <div className="px-2 pt-1 pb-2 text-caption text-muted-foreground">
        {t(($) => $.templates.label)}
      </div>
      {options.map((option) => {
        const picked = option === type;
        return (
          <button
            key={option ?? "none"}
            type="button"
            role="radio"
            aria-checked={picked}
            tabIndex={picked ? 0 : -1}
            data-template={option ?? "none"}
            disabled={disabled}
            onClick={() => onPick(option)}
            className={cn(
              "flex w-full items-center gap-2 rounded-md px-2 py-2 text-left transition-colors hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50",
              picked && "bg-surface-selected",
            )}
          >
            <TypeIcon type={option} className="size-4" />
            <span className="min-w-0">
              <span className="block text-body font-medium">
                {option === null
                  ? t(($) => $.templates.none.name)
                  : t(($) => $.types[option].name)}
              </span>
              <span className="block truncate text-caption text-muted-foreground">
                {option === null
                  ? t(($) => $.templates.none.hint)
                  : t(($) => $.types[option].hint)}
              </span>
            </span>
          </button>
        );
      })}
    </div>
  );
}
