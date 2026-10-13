"use client";

import { Check, X } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import type { AnswerControlProps } from "./control-props";
import { SuggestedBadge } from "./suggested-badge";

/** Two outline buttons; No opens an input for the reason when the asker wants one. */
export function YesNoControl({ question, draft, onDraftChange, note, onNoteChange, disabled }: AnswerControlProps<"yes_no">) {
  const { t } = useT("questions");
  const choices = [
    { choice: "yes", icon: Check, label: t(($) => $.controls.yes) },
    { choice: "no", icon: X, label: t(($) => $.controls.no) },
  ] as const;

  return (
    <div className="flex flex-col gap-2">
      <div className="grid grid-cols-2 gap-2">
        {choices.map(({ choice, icon: Icon, label }) => (
          <Button
            key={choice}
            type="button"
            variant="outline"
            aria-pressed={draft?.choice === choice}
            disabled={disabled}
            className={cn(draft?.choice === choice && "border-foreground")}
            onClick={() => onDraftChange({ choice })}
          >
            <Icon />
            {label}
            {question.suggested_answer?.choice === choice && <SuggestedBadge />}
          </Button>
        ))}
      </div>
      {question.settings.why_on_no && draft?.choice === "no" && (
        <Input
          autoFocus
          value={note}
          disabled={disabled}
          aria-label={t(($) => $.controls.no_reason_label)}
          placeholder={t(($) => $.controls.no_reason_placeholder)}
          onChange={(event) => onNoteChange(event.target.value)}
        />
      )}
    </div>
  );
}
