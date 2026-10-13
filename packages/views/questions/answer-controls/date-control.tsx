"use client";

import { Calendar } from "lucide-react";
import { addDaysDateOnly, toDateOnly, todayDateOnly } from "@multica/core/issues/date";
import { Badge } from "@multica/ui/components/ui/badge";
import { Input } from "@multica/ui/components/ui/input";
import { useT } from "../../i18n";
import type { AnswerControlProps } from "./control-props";
import type { AnswerDraftByType } from "./draft";

const pad = (n: number) => String(n).padStart(2, "0");

/** An RFC 3339 instant as the `datetime-local` value of the viewer's clock. */
function toLocalDateTime(value: string): string {
  const d = new Date(value);
  return `${toDateOnly(d)}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** The instant of a local day's start or end, so a range covers whole days. */
function dayEdge(date: string, edge: "start" | "end"): string {
  return new Date(`${date}T${edge === "start" ? "00:00:00" : "23:59:59"}`).toISOString();
}

/** "YYYY-MM-DD" of the next Friday after today. */
function nextFriday(): string {
  const day = new Date().getDay();
  return addDaysDateOnly(((5 - day + 7) % 7) || 7);
}

/** "YYYY-MM-DD" of the last day of this month. */
function endOfMonth(): string {
  const d = new Date();
  return toDateOnly(new Date(d.getFullYear(), d.getMonth() + 1, 0));
}

/** An input of the asker's kind with a calendar icon; a date has shortcut badges. */
export function DateControl({ question, draft, onDraftChange, disabled }: AnswerControlProps<"date">) {
  const { t } = useT("questions");
  const { kind } = question.settings;

  if (kind === "range") return <DateRangeInputs draft={draft} onDraftChange={onDraftChange} disabled={disabled} />;

  const value = draft?.date ?? (draft?.datetime ? toLocalDateTime(draft.datetime) : "");
  const shortcuts = [
    { label: t(($) => $.controls.date_today), date: todayDateOnly },
    { label: t(($) => $.controls.date_next_friday), date: nextFriday },
    { label: t(($) => $.controls.date_end_of_month), date: endOfMonth },
  ];

  return (
    <div className="flex flex-col gap-2">
      <div className="relative">
        <Calendar aria-hidden className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          type={kind === "date" ? "date" : "datetime-local"}
          value={value}
          disabled={disabled}
          aria-label={kind === "date" ? t(($) => $.controls.date_label) : t(($) => $.controls.datetime_label)}
          className="pl-8"
          onChange={(event) => {
            const typed = event.target.value;
            // An emptied input stays an edit, so Accept suggestion does not come back.
            if (!typed) onDraftChange({});
            else onDraftChange(kind === "date" ? { date: typed } : { datetime: new Date(typed).toISOString() });
          }}
        />
      </div>
      {kind === "date" && (
        <div className="flex flex-wrap gap-1.5">
          {shortcuts.map((shortcut) => (
            <Badge
              key={shortcut.label}
              variant="secondary"
              render={<button type="button" disabled={disabled} />}
              className="cursor-pointer"
              onClick={() => onDraftChange({ date: shortcut.date() })}
            >
              {shortcut.label}
            </Badge>
          ))}
        </div>
      )}
    </div>
  );
}

function DateRangeInputs({
  draft,
  onDraftChange,
  disabled,
}: {
  draft: AnswerDraftByType["date"] | null;
  onDraftChange: (draft: AnswerDraftByType["date"]) => void;
  disabled: boolean;
}) {
  const { t } = useT("questions");
  const days = {
    start: draft?.start ? toDateOnly(new Date(draft.start)) : "",
    end: draft?.end ? toDateOnly(new Date(draft.end)) : "",
  };
  // One day may be picked before the other: the draft keeps the end picked so
  // far, and the card holds Send back until both are there.
  const change = (edge: "start" | "end", date: string) => {
    const next = { ...days, [edge]: date };
    onDraftChange({
      ...(next.start ? { start: dayEdge(next.start, "start") } : {}),
      ...(next.end ? { end: dayEdge(next.end, "end") } : {}),
    });
  };

  return (
    <div className="flex items-center gap-2">
      {(["start", "end"] as const).map((edge) => (
        <div key={edge} className="relative flex-1">
          <Calendar aria-hidden className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="date"
            value={days[edge]}
            disabled={disabled}
            aria-label={edge === "start" ? t(($) => $.controls.range_start) : t(($) => $.controls.range_end)}
            className="pl-8"
            onChange={(event) => change(edge, event.target.value)}
          />
        </div>
      ))}
    </div>
  );
}
