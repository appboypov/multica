"use client";

import { useRef } from "react";
import type { IssueTemplate, IssueType } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { TypeIcon } from "../type-icon";
import { useT } from "../../../i18n";

interface TemplateListProps {
  templates: readonly IssueTemplate[];
  type: IssueType;
  onPick: (type: IssueType) => void;
  className?: string;
}

/**
 * New issue's templates as a radio group: each with its type icon, its name
 * and its hint. Arrow keys move the pick, as in a native radio group.
 */
export function TemplateList({ templates, type, onPick, className }: TemplateListProps) {
  const { t } = useT("issues");
  const listRef = useRef<HTMLDivElement>(null);

  const move = (step: number) => {
    const index = templates.findIndex((template) => template.type === type);
    const next = templates[(index + step + templates.length) % templates.length];
    if (!next) return;
    onPick(next.type);
    listRef.current
      ?.querySelector<HTMLButtonElement>(`[data-template="${next.type}"]`)
      ?.focus();
  };

  return (
    <div
      ref={listRef}
      role="radiogroup"
      aria-label={t(($) => $.templates.label)}
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
      {templates.map((template) => {
        const picked = template.type === type;
        return (
          <button
            key={template.type}
            type="button"
            role="radio"
            aria-checked={picked}
            tabIndex={picked ? 0 : -1}
            data-template={template.type}
            onClick={() => onPick(template.type)}
            className={cn(
              "flex w-full items-center gap-2 rounded-md px-2 py-2 text-left transition-colors hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
              picked && "bg-surface-selected",
            )}
          >
            <TypeIcon type={template.type} className="size-4" />
            <span className="min-w-0">
              <span className="block text-body font-medium">
                {t(($) => $.types[template.type].name)}
              </span>
              <span className="block truncate text-caption text-muted-foreground">
                {t(($) => $.types[template.type].hint)}
              </span>
            </span>
          </button>
        );
      })}
    </div>
  );
}
