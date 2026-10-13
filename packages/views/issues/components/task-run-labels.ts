import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueTasksOptions } from "@multica/core/issues/queries";
import { issueQuestionsOptions } from "@multica/core/questions";
import type { AgentTask } from "@multica/core/types";
import { useT } from "../../i18n";
import { stripMentionMarkdown } from "../utils/strip-mention-markdown";

// Display labels shared by every surface that lists an issue's agent runs:
// the execution log rows and the Runs dialog. They live here rather than in
// either component so the two can't drift — a run must read the same in the
// sidebar and in the timeline it opens.

/**
 * Human label for what caused this run.
 *
 * Primary source: the canonical snapshot taken at task creation time
 * (comment text / autopilot title). Survives source edits/deletes and is
 * information-dense — far better than a structural label.
 *
 * Retry tasks inherit the parent's trigger_summary on the DB side (so the
 * snapshot survives across attempts), but a row that just shows the inherited
 * summary is indistinguishable from its parent. We prepend "Retry #N" when
 * parent_task_id is set so retries are scannable as retries even when their
 * summary is inherited.
 *
 * Fallback chain for legacy tasks created before the snapshot field shipped,
 * OR for sources we don't snapshot (direct assignment / chat): degrade to a
 * short structural label by trigger source. New tasks (post-061 migration)
 * almost always hit the snapshot path.
 */
export function useTriggerText(task: AgentTask): string {
  const { t } = useT("issues");
  const isRetry = !!task.parent_task_id;
  const retryPrefix = isRetry
    ? task.attempt && task.attempt > 1
      ? t(($) => $.execution_log.trigger_retry_attempt_prefix, { attempt: task.attempt })
      : t(($) => $.execution_log.trigger_retry_prefix)
    : "";
  // A wakeup run's stored summary is the rule's instruction; it reads as
  // the wakeup instead. WakeupRunLabel adds the rule's condition.
  if (task.wakeup_id) return retryPrefix + t(($) => $.wakeups.triggered_by_wakeup);

  if (task.trigger_summary) {
    return (
      retryPrefix +
      plainTriggerSummary(task.trigger_summary, t(($) => $.execution_log.trigger_image))
    );
  }
  if (isRetry) {
    return task.attempt && task.attempt > 1
      ? t(($) => $.execution_log.trigger_retry_attempt, { attempt: task.attempt })
      : t(($) => $.execution_log.trigger_retry);
  }
  if (task.autopilot_run_id) return t(($) => $.execution_log.trigger_autopilot);
  if (task.trigger_comment_id) return t(($) => $.execution_log.trigger_comment);
  return t(($) => $.execution_log.trigger_initial);
}

const HTML_ENTITIES: Record<string, string> = {
  amp: "&",
  lt: "<",
  gt: ">",
  quot: '"',
  "#39": "'",
  "#x27": "'",
  nbsp: " ",
};

/**
 * One-line plain text for a trigger snapshot.
 *
 * The snapshot is the comment's raw Markdown cut at ~200 runes (with a
 * trailing "…"), so a list row rendering it verbatim showed `&amp;` for an
 * ampersand the editor escaped, and a comment that opens with a screenshot as
 * `![CleanShot 2026-…` — the cut usually lands inside the image's URL. Images
 * become `imageLabel` (whole or cut), links become their text, mentions their
 * name, entities their character. Decoding is a single pass, so `&amp;lt;`
 * reads `&lt;` exactly as the author typed it.
 */
export function plainTriggerSummary(raw: string, imageLabel: string): string {
  return stripMentionMarkdown(raw)
    .replace(/!\[[^\]]*\]\([^)]*\)/g, imageLabel)
    .replace(/!\[[^\]]*(?:\]\([^)]*)?…?$/, imageLabel)
    .replace(/\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/\[([^\]]*)\]\([^)]*…$/, "$1…")
    .replace(/&(amp|lt|gt|quot|#39|#x27|nbsp);/g, (_, name: string) => HTML_ENTITIES[name] ?? "")
    .replace(/\s+/g, " ")
    .trim();
}

export function useStatusLabel(status: AgentTask["status"]): string {
  const { t } = useT("issues");
  switch (status) {
    case "queued": return t(($) => $.execution_log.status_queued);
    case "dispatched": return t(($) => $.execution_log.status_dispatched);
    case "waiting_local_directory":
      return t(($) => $.execution_log.status_waiting_local_directory);
    case "running": return t(($) => $.execution_log.status_running);
    case "completed": return t(($) => $.execution_log.status_completed);
    case "failed": return t(($) => $.execution_log.status_failed);
    case "cancelled": return t(($) => $.execution_log.status_cancelled);
    default: return status;
  }
}

/**
 * Wall-clock duration of a finished run, or "" when the task never started or
 * never finished. Deliberately not computed for running tasks: the execution
 * log already ticks a live elapsed timer for those, and a second, frozen
 * figure next to it would contradict it.
 */
export function taskDurationMs(task: AgentTask): number | null {
  const startIso = task.started_at ?? task.dispatched_at;
  if (!startIso || !task.completed_at) return null;
  const start = new Date(startIso).getTime();
  const end = new Date(task.completed_at).getTime();
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) return null;
  return end - start;
}

/**
 * Each run's 1-based place among its agent's runs on the issue, by
 * `created_at`: run #3 is the third run that agent had on the issue.
 */
export function runNumbersByTask(tasks: readonly AgentTask[]): ReadonlyMap<string, number> {
  const byAgent = new Map<string, AgentTask[]>();
  for (const task of tasks) {
    const runs = byAgent.get(task.agent_id);
    if (runs) runs.push(task);
    else byAgent.set(task.agent_id, [task]);
  }
  const numbers = new Map<string, number>();
  for (const runs of byAgent.values()) {
    runs
      .sort((a, b) => a.created_at.localeCompare(b.created_at) || a.id.localeCompare(b.id))
      .forEach((run, index) => numbers.set(run.id, index + 1));
  }
  return numbers;
}

/**
 * The run row of a terminal run that asked a question: "ended run #N with a
 * question", N its place among its agent's runs on the issue, and whether one
 * of its questions still waits for an answer. Null for any other run.
 */
export function useAskedQuestionRun(task: AgentTask): { label: string; waiting: boolean } | null {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const terminal = task.status === "completed" || task.status === "failed" || task.status === "cancelled";
  const { data: questions } = useQuery({
    ...issueQuestionsOptions(wsId, task.issue_id),
    enabled: terminal && !!wsId && !!task.issue_id,
  });
  const { data: tasks } = useQuery({ ...issueTasksOptions(task.issue_id), enabled: terminal && !!task.issue_id });
  return useMemo(() => {
    if (!terminal || !questions || !tasks) return null;
    const asked = questions.filter((question) => question.asked_in_task_id === task.id);
    const number = runNumbersByTask(tasks).get(task.id);
    if (asked.length === 0 || number === undefined) return null;
    return {
      label: t(($) => $.inline_run.ended_with_question, { number }),
      waiting: asked.some((question) => question.status === "open"),
    };
  }, [terminal, questions, tasks, task.id, t]);
}
