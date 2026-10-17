"use client";

import { Check, CalendarClock, Settings } from "lucide-react";
import { paths, useWorkspaceSlug } from "@multica/core/paths";
import { canMarkTodoDone, type Todo } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Badge } from "@multica/ui/components/ui/badge";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../../common/actor-avatar";
import { handleRowActivationKey } from "../../common/row-actions-menu";
import { useT } from "../../i18n";
import { useStatusLabel } from "../../issues/utils/status-label";
import { resolveClickIntent, useIntentNavigate } from "../../navigation";
import { useTimeAgo } from "./inbox-list-item";
import { SYSTEM_TODO_KINDS, TodoKindIcon, useTodoKindLabel } from "./todo-display";

/**
 * One to-do: who it waits on, the issue (or the autopilot, or the prompt), its
 * age or Overdue, and the kind with the actor. Rows carry no unread dot: every
 * row in the Inbox waits for the member. Mark done takes the age's place on
 * hover.
 */
export function TodoListItem({
  todo,
  isSelected,
  onClick,
  onMarkDone,
}: {
  todo: Todo;
  isSelected: boolean;
  onClick: () => void;
  onMarkDone: () => void;
}) {
  const { t } = useT("inbox");
  const timeAgo = useTimeAgo();
  const kindLabel = useTodoKindLabel();
  const { getActorName } = useActorName();
  const statusLabelOf = useStatusLabel(todo.workspace_id);
  // Null-safe slug: without one the modifier-click affordance stays off.
  const slug = useWorkspaceSlug();
  const issueHref = slug && todo.issue_id ? paths.workspace(slug).issueDetail(todo.issue_id) : null;
  const intentNavigate = useIntentNavigate();
  // Who the to-do waits on; system problems and actor-less rows show the settings mark.
  const actor = SYSTEM_TODO_KINDS[todo.kind] !== true && todo.actor_type && todo.actor_id
    ? { type: todo.actor_type, id: todo.actor_id }
    : null;
  const markDoneLabel = t(($) => $.todos.mark_done);
  const meta = [
    kindLabel(todo),
    todo.kind === "assigned" && todo.issue_status ? statusLabelOf(todo.issue_status) : null,
  ].filter(Boolean).join(" · ");

  return (
    // A div, not a <button>: the row carries its own Mark done control.
    <div
      role="button"
      tabIndex={0}
      aria-current={isSelected || undefined}
      data-todo-id={todo.id}
      onClick={(e) => {
        // A modifier click on an issue's to-do opens the issue as its own tab.
        if (issueHref) {
          const intent = resolveClickIntent(e);
          if (intent !== "push") {
            intentNavigate(issueHref, intent);
            return;
          }
        }
        onClick();
      }}
      onKeyDown={(e) => handleRowActivationKey(e, onClick)}
      onAuxClick={(e) => {
        if (e.defaultPrevented || e.button !== 1 || !issueHref) return;
        e.preventDefault();
        intentNavigate(issueHref, "background-tab");
      }}
      className={cn(
        "group flex w-full cursor-default select-none items-start gap-3 rounded-md px-2 py-2.5 text-left outline-none transition-colors focus-visible:ring-1 focus-visible:ring-ring",
        isSelected ? "bg-accent" : "hover:bg-accent/50",
      )}
    >
      {actor ? (
        <ActorAvatar actorType={actor.type} actorId={actor.id} size="sm" className="mt-0.5" enableHoverCard />
      ) : (
        <span aria-hidden className="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
          <Settings className="size-3" />
        </span>
      )}
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-2">
          <span className="truncate text-body font-medium">{todo.title}</span>
          <div className="flex shrink-0 items-center gap-1">
            {todo.overdue && (
              <Badge variant="destructive">
                <CalendarClock />
                {t(($) => $.todos.overdue)}
              </Badge>
            )}
            {!todo.overdue && (
              <span className={cn("text-caption text-muted-foreground", canMarkTodoDone(todo.kind) && "[@media(hover:hover)]:group-hover:hidden [@media(hover:hover)]:group-focus-within:hidden")}>
                {timeAgo(todo.created_at)}
              </span>
            )}
            {/* Pointer-only: revealed on hover and on keyboard focus in the
                row. Touch has neither; the detail head carries Mark done. */}
            {canMarkTodoDone(todo.kind) && (
            <button
              type="button"
              title={markDoneLabel}
              aria-label={markDoneLabel}
              onClick={(e) => {
                e.stopPropagation();
                onMarkDone();
              }}
              className="hidden rounded-xs p-0.5 text-muted-foreground outline-none hover:bg-accent hover:text-foreground focus-visible:ring-1 focus-visible:ring-ring [@media(hover:hover)]:group-hover:inline-flex [@media(hover:hover)]:group-focus-within:inline-flex"
            >
              <Check className="size-3.5" />
            </button>
            )}
          </div>
        </div>
        <p className="mt-0.5 flex min-w-0 items-center gap-1.5 text-caption text-muted-foreground">
          <TodoKindIcon todo={todo} />
          <span className="truncate">
            {meta}
            {actor && <span className="text-faint-foreground">{" · "}{getActorName(actor.type, actor.id)}</span>}
          </span>
        </p>
      </div>
    </div>
  );
}
