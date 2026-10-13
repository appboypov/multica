"use client";

import { Check, X } from "lucide-react";
import type { FactCheck } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import type { AnswerControlProps } from "./control-props";

/** Per fact **Correct** or **Not correct**; Not correct opens an input for the correction. */
export function ConfirmFactsControl({ question, draft, onDraftChange, disabled }: AnswerControlProps<"confirm_facts">) {
  const { t } = useT("questions");
  const facts = draft?.facts ?? question.settings.facts.map(() => null);
  const mark = (index: number, check: FactCheck | null) => {
    const next = facts.map((fact, i) => (i === index ? check : fact));
    onDraftChange({ facts: next });
  };

  return (
    <div className="flex flex-col gap-2">
      {question.settings.facts.map((fact, index) => {
        const check = facts[index] ?? null;
        const wrong = check !== null && !check.correct;
        const correction = check && !check.correct ? check.correction : "";
        return (
          <div
            key={fact}
            role="group"
            aria-label={fact}
            className={cn("flex flex-col gap-2 rounded-lg border p-2.5", wrong && !correction.trim() ? "border-destructive" : "border-border")}
          >
            <div className="flex items-center gap-2">
              <span className="min-w-0 flex-1 text-body">{fact}</span>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                aria-pressed={check?.correct === true}
                disabled={disabled}
                className={cn(check?.correct === true ? "bg-muted text-success" : "text-muted-foreground")}
                onClick={() => mark(index, { correct: true })}
              >
                <Check />
                {t(($) => $.controls.correct)}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                aria-pressed={wrong}
                disabled={disabled}
                className={cn(wrong ? "bg-muted text-destructive" : "text-muted-foreground")}
                onClick={() => mark(index, { correct: false, correction })}
              >
                <X />
                {t(($) => $.controls.not_correct)}
              </Button>
            </div>
            {wrong && (
              <Input
                autoFocus
                value={correction}
                disabled={disabled}
                aria-invalid={!correction.trim()}
                aria-label={t(($) => $.controls.correction_label, { fact })}
                placeholder={t(($) => $.controls.correction_placeholder)}
                onChange={(event) => mark(index, { correct: false, correction: event.target.value })}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
