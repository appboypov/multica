"use client";

import { AlarmClock } from "lucide-react";
import { useActorName } from "@multica/core/workspace/hooks";
import type { TimelineEntry } from "@multica/core/types";
import { useT } from "../../i18n";

/** The activity Wake writes, with details `{ agent_id, answer_count, task_id }`. */
export const QUESTION_WAKE_ACTION = "question_wake";

/**
 * "Pieter was woken by Brian with 1 answer". The woken agent is the subject,
 * so the row shows this sentence in place of the actor's name.
 */
export function QuestionWakeActivityText({ entry }: { entry: TimelineEntry }) {
  const { t } = useT("issues");
  const { getActorName } = useActorName();
  const details = entry.details ?? {};
  const agentId = typeof details.agent_id === "string" ? details.agent_id : "";
  const count = Number(details.answer_count) || 0;
  return (
    <>
      {t(($) => $.activity.question_woken, {
        agent: getActorName("agent", agentId),
        member: entry.actor_name || getActorName(entry.actor_type, entry.actor_id),
        count,
      })}
    </>
  );
}

export function QuestionWakeActivityIcon() {
  return <AlarmClock aria-hidden className="h-4 w-4 shrink-0 text-muted-foreground" />;
}
