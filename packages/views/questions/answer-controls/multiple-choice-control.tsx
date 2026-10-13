"use client";

import { toggleOption } from "./draft";
import type { AnswerControlProps } from "./control-props";
import { OptionRow } from "./option-row";

/** Checkbox options; the card's line says when the picks miss min or max. */
export function MultipleChoiceControl({ question, draft, onDraftChange, disabled }: AnswerControlProps<"multiple_choice">) {
  const suggested = question.suggested_answer?.options ?? [];
  return (
    <div className="flex flex-col gap-2">
      {question.settings.options.map((option, index) => (
        <OptionRow
          key={option}
          mark="checkbox"
          selected={draft?.options.includes(option) ?? false}
          suggested={suggested.includes(option)}
          shortcut={index < 9 ? index + 1 : undefined}
          disabled={disabled}
          onSelect={() => onDraftChange(toggleOption(question, draft, option))}
        >
          {option}
        </OptionRow>
      ))}
    </div>
  );
}
