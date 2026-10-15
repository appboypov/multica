"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { CircleX, List, OctagonPause, Play } from "lucide-react";
import { useAuthStore } from "@multica/core/auth";
import { useUpdateAutopilot } from "@multica/core/autopilots";
import { issuePullRequestsOptions } from "@multica/core/github";
import { useIssueStatuses } from "@multica/core/issue-statuses/hooks";
import { issueTasksOptions } from "@multica/core/issues/queries";
import { useWorkspacePaths } from "@multica/core/paths";
import type { TimelineEntry, Todo } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../../common/actor-avatar";
import { ReadonlyContent } from "../../editor";
import { useT, useTimeAgo } from "../../i18n";
import { CommentInput } from "../../issues/components/comment-input";
import { PullRequestList } from "../../issues/components/pull-request-list";
import { ReplyInput } from "../../issues/components/reply-input";
import { StatusIcon } from "../../issues/components/status-icon";
import { runNumbersByTask } from "../../issues/components/task-run-labels";
import { useIssueTimeline } from "../../issues/hooks/use-issue-timeline";
import { useStatusLabel } from "../../issues/utils/status-label";
import { useNavigation } from "../../navigation";
import { useIssueLimitUpgradePrompt } from "../../modals/use-issue-limit-upgrade-prompt";
import { AutopilotQuotaNotice } from "./autopilot-quota-notice";
import { QuickCreateRecovery } from "./quick-create-recovery";
import { TodoKindIcon, useTodoKindLabel } from "./todo-display";

/** Comments a reader can see: deleted tombstones hold no text to show. */
function visibleComments(timeline: readonly TimelineEntry[]): TimelineEntry[] {
  return timeline.filter((entry) => entry.type === "comment" && !entry.deleted_at && !!entry.content?.trim());
}

/** One comment, read-only: its author, age and text. `outlined` marks the comment the to-do is about. */
function TodoComment({ entry, outlined = false }: { entry: TimelineEntry; outlined?: boolean }) {
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  return (
    <div
      data-comment-id={entry.id}
      className={cn("flex flex-col gap-1.5 rounded-xl border bg-card p-3.5", outlined && "border-brand")}
    >
      <div className="flex items-center gap-2">
        <ActorAvatar actorType={entry.actor_type} actorId={entry.actor_id} name={entry.actor_name} avatarUrl={entry.actor_avatar_url} size="sm" enableHoverCard />
        <span className="text-body font-medium">{entry.actor_name ?? getActorName(entry.actor_type, entry.actor_id)}</span>
        <span className="text-caption text-muted-foreground">{timeAgo(entry.created_at)}</span>
      </div>
      <ReadonlyContent content={entry.content ?? ""} attachments={entry.attachments} className="text-body" />
    </div>
  );
}

/** Mention: the comment before it, the mentioning comment outlined, and a reply box. */
export function TodoMentionBody({ todo, issueId }: { todo: Todo; issueId: string }) {
  const { t } = useT("inbox");
  const userId = useAuthStore((s) => s.user?.id);
  const { getActorName } = useActorName();
  const { timeline, loading, submitReply } = useIssueTimeline(issueId, userId);
  const { mention, before } = useMemo(() => {
    const comments = visibleComments(timeline);
    const index = comments.findIndex((entry) => entry.id === todo.comment_id);
    if (index < 0) return { mention: null, before: null };
    const found = comments[index]!;
    // The comment it answers: the latest earlier one in its thread, or the
    // latest earlier top-level comment.
    const earlier = comments.slice(0, index).reverse();
    const previous = found.parent_id
      ? earlier.find((entry) => entry.id === found.parent_id || entry.parent_id === found.parent_id)
      : earlier.find((entry) => !entry.parent_id);
    return { mention: found, before: previous ?? null };
  }, [timeline, todo.comment_id]);

  if (loading) return <Skeleton className="h-32 w-full rounded-xl" />;
  if (!mention) {
    return <p className="text-caption text-muted-foreground">{t(($) => $.todos.comment_unavailable)}</p>;
  }
  const author = mention.actor_name ?? getActorName(mention.actor_type, mention.actor_id);
  return (
    <div className="flex flex-col gap-4">
      {before && <TodoComment entry={before} />}
      <TodoComment entry={mention} outlined />
      {userId && (
        <ReplyInput
          issueId={issueId}
          parentId={mention.parent_id ?? mention.id}
          placeholder={t(($) => $.todos.reply_to, { name: author })}
          avatarType="member"
          avatarId={userId}
          onSubmit={(content, attachmentIds, suppressAgentIds, steerTaskIds) =>
            submitReply(mention.parent_id ?? mention.id, content, attachmentIds, suppressAgentIds, steerTaskIds)}
        />
      )}
    </div>
  );
}

/** The issue's status with what ties the member to it: "created by you" or "assigned to you". */
function TodoIssueStatus({ todo, relation }: { todo: Todo; relation: string }) {
  const { categoryOf, colorOf, iconOf } = useIssueStatuses(todo.workspace_id);
  const statusLabelOf = useStatusLabel(todo.workspace_id);
  if (!todo.issue_status) return null;
  return (
    <div className="flex items-center gap-1.5 text-body">
      <StatusIcon
        status={todo.issue_status}
        category={categoryOf(todo.issue_status)}
        color={colorOf(todo.issue_status)}
        icon={iconOf(todo.issue_status)}
        className="size-4 shrink-0"
      />
      <span className="font-medium">{statusLabelOf(todo.issue_status)}</span>
      <span className="text-caption text-muted-foreground">· {relation}</span>
    </div>
  );
}

/**
 * Waiting for your review and Assigned to you: the issue's status, the pull
 * requests for a review, the latest comment and a comment box.
 */
export function TodoIssueBody({ todo, issueId }: { todo: Todo; issueId: string }) {
  const { t } = useT("inbox");
  const userId = useAuthStore((s) => s.user?.id);
  const { timeline, submitComment } = useIssueTimeline(issueId, userId);
  const isReview = todo.kind === "review";
  const { data: pullRequests } = useQuery({ ...issuePullRequestsOptions(issueId), enabled: isReview });
  const latest = useMemo(() => visibleComments(timeline).at(-1) ?? null, [timeline]);
  return (
    <div className="flex flex-col gap-4">
      <TodoIssueStatus
        todo={todo}
        relation={isReview ? t(($) => $.todos.created_by_you) : t(($) => $.todos.assigned_to_you)}
      />
      {isReview && !!pullRequests?.pull_requests.length && (
        <div className="rounded-xl border bg-card p-2">
          <PullRequestList issueId={issueId} identifier={todo.issue_identifier ?? ""} />
        </div>
      )}
      {latest && <TodoComment entry={latest} />}
      <CommentInput key={issueId} issueId={issueId} onSubmit={submitComment} />
    </div>
  );
}

/** Agent blocked and Run failed: the run with its state, the agent's last comment and a comment box. */
export function TodoRunBody({ todo, issueId }: { todo: Todo; issueId: string }) {
  const { t } = useT("inbox");
  const timeAgo = useTimeAgo();
  const userId = useAuthStore((s) => s.user?.id);
  const { getActorName } = useActorName();
  const { timeline, submitComment } = useIssueTimeline(issueId, userId);
  const { data: tasks } = useQuery(issueTasksOptions(issueId));
  const failed = todo.kind === "run_failed";
  const agentId = todo.actor_type === "agent" ? todo.actor_id : null;
  const runNumber = useMemo(() => {
    if (!tasks) return undefined;
    // The run the to-do names, else the agent's latest run on the issue.
    const task = tasks.find((entry) => entry.id === todo.task_id)
      ?? tasks.filter((entry) => entry.agent_id === agentId).sort((a, b) => b.created_at.localeCompare(a.created_at))[0];
    return task ? runNumbersByTask(tasks).get(task.id) : undefined;
  }, [tasks, todo.task_id, agentId]);
  const lastComment = useMemo(() => {
    const comments = visibleComments(timeline);
    return comments.filter((entry) => entry.actor_id === agentId).at(-1) ?? comments.at(-1) ?? null;
  }, [timeline, agentId]);
  const StateIcon = failed ? CircleX : OctagonPause;
  const runLabel = runNumber === undefined
    ? t(($) => $.todos.run_label_plain)
    : t(($) => $.todos.run_label, { number: runNumber });

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2 px-1">
        {todo.actor_type && todo.actor_id && (
          <>
            <ActorAvatar actorType={todo.actor_type} actorId={todo.actor_id} size="sm" enableHoverCard />
            <span className="text-body font-medium">{getActorName(todo.actor_type, todo.actor_id)}</span>
          </>
        )}
        <span className="text-caption text-muted-foreground">{runLabel}</span>
        <Badge variant="destructive">
          <StateIcon />
          {failed ? t(($) => $.todos.failed_badge) : t(($) => $.todos.blocked_badge)}
        </Badge>
        <span className="ml-auto text-caption text-muted-foreground">{timeAgo(todo.created_at)}</span>
      </div>
      {lastComment && <TodoComment entry={lastComment} />}
      <CommentInput key={issueId} issueId={issueId} onSubmit={submitComment} />
    </div>
  );
}

/** System problems: what went wrong and the action that solves it. */
export function TodoSystemBody({ todo }: { todo: Todo }) {
  const { t } = useT("inbox");
  const kindLabel = useTodoKindLabel();
  const { push } = useNavigation();
  const wsPaths = useWorkspacePaths();
  const updateAutopilot = useUpdateAutopilot();
  const showAutopilotQuotaRecoveryPrompt = useIssueLimitUpgradePrompt("autopilot_quota");
  const originalPrompt = typeof todo.details.original_prompt === "string" ? todo.details.original_prompt : "";
  const errorText = typeof todo.details.error === "string" ? todo.details.error : "";
  const autopilotId = todo.autopilot_id;

  return (
    <div className="flex flex-col gap-3 rounded-xl border bg-card p-4">
      <div className="flex items-center gap-2">
        <TodoKindIcon todo={todo} className="size-4" />
        <span className="text-body font-medium">{kindLabel(todo)}</span>
      </div>
      {todo.kind === "quota_reached" ? (
        <AutopilotQuotaNotice item={todo} onOpenRecovery={showAutopilotQuotaRecoveryPrompt} />
      ) : (
        <>
          {(todo.body || errorText) && (
            <p className="whitespace-pre-wrap text-body leading-relaxed text-muted-foreground">{todo.body || errorText}</p>
          )}
          {todo.kind === "quick_create_failed" && originalPrompt && (
            <div className="rounded-md border bg-muted/40 p-3">
              <p className="text-caption font-medium text-muted-foreground">{t(($) => $.detail.original_input)}</p>
              <p className="mt-1 whitespace-pre-wrap text-body">{originalPrompt}</p>
            </div>
          )}
          <div className="flex flex-wrap items-center gap-2">
            {todo.kind === "autopilot_paused" && autopilotId && (
              <>
                <Button
                  size="sm"
                  disabled={updateAutopilot.isPending}
                  onClick={() =>
                    updateAutopilot.mutate(
                      { id: autopilotId, status: "active" },
                      { onError: () => toast.error(t(($) => $.errors.resume_autopilot_failed)) },
                    )}
                >
                  <Play />
                  {t(($) => $.todos.resume_autopilot)}
                </Button>
                <Button size="sm" variant="ghost" onClick={() => push(wsPaths.autopilotDetail(autopilotId))}>
                  <List />
                  {t(($) => $.todos.view_runs)}
                </Button>
              </>
            )}
            {todo.kind === "quick_create_failed" && <QuickCreateRecovery details={todo.details} retryable />}
          </div>
        </>
      )}
    </div>
  );
}
