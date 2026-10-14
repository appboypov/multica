"use client";

import { Calendar, CirclePause, ClockAlert, Users } from "lucide-react";
import { formatDateOnly, isPastDateOnly, todayDateOnly } from "@multica/core/issues/date";
import type { AnswerPolicy } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { cn } from "@multica/ui/lib/utils";
import { useLocale, useT } from "../i18n";

export interface QuestionBadgesProps {
  blocksWork: boolean;
  /** Date-only, YYYY-MM-DD. */
  dueDate: string | null;
  /** Only an open question is overdue. */
  open: boolean;
  /** With more than one answerer the badge reads "any one of 2" or "each of 3". */
  policy?: AnswerPolicy;
  answererCount?: number;
  className?: string;
}

/**
 * Blocks work, the answer policy, and the due date: warning on the due day,
 * Overdue past it while the question is open. Shared by the Questions page's
 * rows and the question card.
 */
export function QuestionBadges({ blocksWork, dueDate, open, policy, answererCount = 0, className }: QuestionBadgesProps) {
  const { t } = useT("questions");
  const locale = useLocale();
  const overdue = open && isPastDateOnly(dueDate);
  const dueToday = dueDate !== null && dueDate === todayDateOnly();

  return (
    <span className={cn("flex shrink-0 items-center gap-1.5", className)}>
      {blocksWork && (
        <Badge variant="destructive" className="border-destructive">
          <CirclePause />
          {t(($) => $.card.blocks_work)}
        </Badge>
      )}
      {policy && answererCount > 1 && (
        <Badge variant="secondary">
          <Users />
          {policy === "each"
            ? t(($) => $.badges.policy_each, { count: answererCount })
            : t(($) => $.badges.policy_any_one, { count: answererCount })}
        </Badge>
      )}
      {overdue ? (
        <Badge variant="destructive" className="border-destructive">
          <ClockAlert />
          {t(($) => $.card.overdue)}
        </Badge>
      ) : dueDate && (
        <Badge variant="secondary" className={cn(open && dueToday && "bg-warning/15 text-warning")}>
          <Calendar />
          {dueToday
            ? t(($) => $.card.due_today)
            : t(($) => $.card.due_date, { date: formatDateOnly(dueDate, { month: "short", day: "numeric" }, locale) })}
        </Badge>
      )}
    </span>
  );
}
