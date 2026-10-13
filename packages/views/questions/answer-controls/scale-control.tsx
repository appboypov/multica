"use client";

import { Button } from "@multica/ui/components/ui/button";
import type { AnswerControlProps } from "./control-props";

/** One button per point, the picked one primary, the end labels below. */
export function ScaleControl({ question, draft, onDraftChange, disabled }: AnswerControlProps<"scale">) {
  const { min, max, min_label, max_label } = question.settings;
  const points = Array.from({ length: Math.max(0, max - min + 1) }, (_, i) => min + i);
  return (
    <div className="flex flex-col gap-1.5">
      <div role="radiogroup" className="flex gap-1.5">
        {points.map((point) => (
          <Button
            key={point}
            type="button"
            role="radio"
            aria-checked={draft?.point === point}
            variant={draft?.point === point ? "default" : "outline"}
            disabled={disabled}
            className="min-w-0 flex-1"
            onClick={() => onDraftChange({ point })}
          >
            {point}
          </Button>
        ))}
      </div>
      {(min_label || max_label) && (
        <div className="flex justify-between gap-4 text-caption text-muted-foreground">
          <span>{min_label}</span>
          <span className="text-right">{max_label}</span>
        </div>
      )}
    </div>
  );
}
