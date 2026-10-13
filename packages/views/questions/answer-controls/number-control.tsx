"use client";

import { useState } from "react";
import { Input } from "@multica/ui/components/ui/input";
import { useT } from "../../i18n";
import type { AnswerControlProps } from "./control-props";

/** An input with the unit beside it and the allowed range on the right. */
export function NumberControl({ question, draft, onDraftChange, disabled }: AnswerControlProps<"number">) {
  const { t } = useT("questions");
  const { unit, min, max } = question.settings;
  // The typed text, since "", "-" and "1." are on the way to a number but not one yet.
  const [text, setText] = useState(draft && draft.number !== null ? String(draft.number) : "");
  const range =
    min !== undefined && max !== undefined
      ? t(($) => $.controls.number_range, { min, max })
      : min !== undefined
        ? t(($) => $.controls.number_minimum, { min })
        : max !== undefined
          ? t(($) => $.controls.number_maximum, { max })
          : null;

  return (
    <div className="flex items-center gap-3">
      <Input
        type="number"
        inputMode="decimal"
        value={text}
        min={min}
        max={max}
        disabled={disabled}
        aria-label={t(($) => $.controls.number_label)}
        className="w-32"
        onChange={(event) => {
          setText(event.target.value);
          const number = event.target.value.trim() === "" ? Number.NaN : Number(event.target.value);
          // Text that is no number stays an edit, so Accept suggestion does not come back.
          onDraftChange({ number: Number.isFinite(number) ? number : null });
        }}
      />
      {unit && <span className="text-body text-muted-foreground">{unit}</span>}
      {range && <span className="ml-auto text-caption text-muted-foreground">{range}</span>}
    </div>
  );
}
