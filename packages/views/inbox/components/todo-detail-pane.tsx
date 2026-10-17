"use client";

import type { ReactNode } from "react";
import { ArrowUpRight, Check } from "lucide-react";
import { useWorkspacePaths } from "@multica/core/paths";
import { canMarkTodoDone, type Todo } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useT, useTimeAgo } from "../../i18n";
import { useNavigation } from "../../navigation";
import { useStatusLabel } from "../../issues/utils/status-label";
import { InboxQuestionBody } from "./inbox-question-pane";
import { TodoIssueBody, TodoMentionBody, TodoRunBody, TodoSystemBody } from "./todo-detail-bodies";
import { TodoKindIcon, useTodoKindLabel } from "./todo-display";
import { TodoCheckBody } from "./todo-check-body";
import { TodoInstructionBody } from "./todo-instruction-body";

/** Where "Open on …" lands on the issue: the question's card or the comment. */
export interface TodoIssueTarget {
  questionId?: string;
  commentId?: string;
}

/** The line that says what makes the to-do leave the Inbox. */
function useLeavesLine(wsId: string) {
  const { t } = useT("inbox");
  const statusLabelOf = useStatusLabel(wsId);
  return (todo: Todo): string => {
    const identifier = todo.issue_identifier ?? todo.issue_title ?? todo.title;
    switch (todo.kind) {
      case "instruction_change":
        return t(($) => $.todos.leaves.instruction_change);
      case "check":
      case "question":
        return t(($) => $.todos.leaves.question);
      case "mention":
        return t(($) => $.todos.leaves.mention, { identifier });
      case "assigned":
        return t(($) => $.todos.leaves.assigned, { identifier, status: statusLabelOf(todo.issue_status ?? "") });
      case "agent_blocked":
        return t(($) => $.todos.leaves.agent_blocked, { identifier });
      case "run_failed":
        return t(($) => $.todos.leaves.run_failed, { identifier });
      case "review":
        return t(($) => $.todos.leaves.review, { identifier });
      case "autopilot_paused":
        return t(($) => $.todos.leaves.autopilot_paused);
      case "quota_reached":
        return t(($) => $.todos.leaves.quota_reached);
      case "quick_create_failed":
        return t(($) => $.todos.leaves.quick_create_failed);
      default:
        return t(($) => $.todos.leaves.other);
    }
  };
}

/** What the member acts on, per kind. */
function TodoBody({ todo, onOpenIssue }: { todo: Todo; onOpenIssue: (target: TodoIssueTarget) => void }) {
  const { issue_id: issueId, question_id: todoQuestionId } = todo;
  if (todo.kind === "check") return <TodoCheckBody key={todo.id} todo={todo} />;
  if (todo.kind === "instruction_change") return <TodoInstructionBody key={todo.id} todo={todo} />;
  if (todo.kind === "question" && todoQuestionId) {
    return (
      <InboxQuestionBody
        issueId={issueId}
        questionId={todoQuestionId}
        fallbackIdentifier={todo.issue_identifier ?? todo.title}
        onOpenIssue={(questionId) => onOpenIssue({ questionId: questionId ?? todoQuestionId })}
      />
    );
  }
  if (todo.kind === "mention" && issueId) return <TodoMentionBody key={todo.id} todo={todo} issueId={issueId} />;
  if ((todo.kind === "review" || todo.kind === "assigned") && issueId) return <TodoIssueBody todo={todo} issueId={issueId} />;
  if ((todo.kind === "agent_blocked" || todo.kind === "run_failed") && issueId) return <TodoRunBody todo={todo} issueId={issueId} />;
  return <TodoSystemBody todo={todo} />;
}

/**
 * The detail of one to-do: the kind and age with Open on the issue (or the
 * autopilot) and Mark done in its head, the issue, the line that says when it
 * leaves, and the body the member acts on.
 */
export function TodoDetailPane({
  todo,
  onMarkDone,
  onOpenIssue,
  leadingAction,
  className,
}: {
  todo: Todo;
  onMarkDone: () => void;
  /** Opens the to-do's issue in the pane, landing on `target`. */
  onOpenIssue: (target: TodoIssueTarget) => void;
  /** The compact layout's way back to the list. */
  leadingAction?: ReactNode;
  className?: string;
}) {
  const { t } = useT("inbox");
  const timeAgo = useTimeAgo();
  const kindLabel = useTodoKindLabel();
  const leavesLine = useLeavesLine(todo.workspace_id);
  const { push } = useNavigation();
  const wsPaths = useWorkspacePaths();
  const autopilotId = todo.autopilot_id;
  const issueTarget: TodoIssueTarget = todo.kind === "question" && todo.question_id
    ? { questionId: todo.question_id }
    : todo.comment_id ? { commentId: todo.comment_id } : {};

  return (
    <div className={cn("flex min-h-0 flex-1 flex-col", className)} data-testid="todo-detail" data-todo-id={todo.id}>
      <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
        {leadingAction}
        <TodoKindIcon todo={todo} className="size-4" />
        <span className="truncate text-body font-medium">{kindLabel(todo)}</span>
        <span className="shrink-0 text-caption text-muted-foreground">· {timeAgo(todo.created_at)}</span>
        <span className="ml-auto flex shrink-0 items-center gap-1">
          {todo.issue_id ? (
            <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" onClick={() => onOpenIssue(issueTarget)}>
              <ArrowUpRight />
              {t(($) => $.question_pane.open_on, { identifier: todo.issue_identifier ?? todo.title })}
            </Button>
          ) : autopilotId ? (
            <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" onClick={() => push(wsPaths.autopilotDetail(autopilotId))}>
              <ArrowUpRight />
              {t(($) => $.todos.open_autopilot)}
            </Button>
          ) : null}
          {canMarkTodoDone(todo.kind) && (
          <Button type="button" variant="outline" size="sm" onClick={onMarkDone}>
            <Check />
            {t(($) => $.todos.mark_done)}
          </Button>
          )}
        </span>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-3xl flex-col gap-4">
          <div className="flex flex-col gap-0.5">
            {todo.issue_identifier && <span className="text-caption text-muted-foreground">{todo.issue_identifier}</span>}
            <h2 className="text-title font-semibold">{todo.issue_title ?? todo.title}</h2>
            <p className="mt-1 text-caption text-faint-foreground">{leavesLine(todo)}</p>
          </div>
          <TodoBody todo={todo} onOpenIssue={onOpenIssue} />
        </div>
      </div>
    </div>
  );
}
