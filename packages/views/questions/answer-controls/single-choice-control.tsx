"use client";

import { Input } from "@multica/ui/components/ui/input";
import { useT } from "../../i18n";
import type { AnswerControlProps } from "./control-props";
import { OptionRow } from "./option-row";

/** Radio options; Other opens an input whose text is the answer. */
export function SingleChoiceControl({ question, draft, onDraftChange, disabled }: AnswerControlProps<"single_choice">) {
  const { t } = useT("questions");
  const suggested = question.suggested_answer && "option" in question.suggested_answer
    ? question.suggested_answer.option
    : null;
  const other = draft && "other" in draft ? draft.other : null;

  return (
    <div className="flex flex-col gap-2">
      {question.settings.options.map((option, index) => (
        <OptionRow
          key={option}
          mark="radio"
          selected={!!draft && "option" in draft && draft.option === option}
          suggested={option === suggested}
          shortcut={index < 9 ? index + 1 : undefined}
          disabled={disabled}
          onSelect={() => onDraftChange({ option })}
        >
          {option}
        </OptionRow>
      ))}
      {question.settings.allow_other && (
        <OptionRow
          mark="radio"
          selected={other !== null}
          shortcut={question.settings.options.length < 9 ? question.settings.options.length + 1 : undefined}
          disabled={disabled}
          onSelect={() => onDraftChange({ other: other ?? "" })}
        >
          {t(($) => $.controls.other)}
        </OptionRow>
      )}
      {other !== null && (
        <Input
          autoFocus
          value={other}
          disabled={disabled}
          aria-label={t(($) => $.controls.other_label)}
          placeholder={t(($) => $.controls.other_placeholder)}
          onChange={(event) => onDraftChange({ other: event.target.value })}
        />
      )}
    </div>
  );
}
