"use client";

import { useCallback } from "react";
import {
  AtSign,
  BadgeCheck,
  CirclePause,
  CircleX,
  Gauge,
  GitPullRequest,
  Info,
  MessageCircleQuestionMark,
  OctagonPause,
  TriangleAlert,
  UserRoundCheck,
  type LucideIcon,
} from "lucide-react";
import type { Todo, TodoKind } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";

/** Kinds no person stands behind: their row shows a settings mark, not an avatar. */
export const SYSTEM_TODO_KINDS: Partial<Record<string, true>> = {
  autopilot_paused: true,
  quota_reached: true,
  quick_create_failed: true,
};

const KIND_ICONS: Record<TodoKind, { Icon: LucideIcon; className: string }> = {
  question: { Icon: MessageCircleQuestionMark, className: "text-brand" },
  mention: { Icon: AtSign, className: "text-brand" },
  assigned: { Icon: UserRoundCheck, className: "text-muted-foreground" },
  agent_blocked: { Icon: OctagonPause, className: "text-destructive" },
  run_failed: { Icon: CircleX, className: "text-destructive" },
  autopilot_paused: { Icon: CirclePause, className: "text-warning" },
  quota_reached: { Icon: Gauge, className: "text-warning" },
  quick_create_failed: { Icon: TriangleAlert, className: "text-warning" },
  review: { Icon: GitPullRequest, className: "text-info" },
};

function isKnownKind(kind: string): kind is TodoKind {
  return Object.hasOwn(KIND_ICONS, kind);
}

/** An approval is a question whose answer type is `approval`; it has its own icon and label. */
function isApproval(todo: Pick<Todo, "kind" | "question_answer_type">): boolean {
  return todo.kind === "question" && todo.question_answer_type === "approval";
}

/** The kind's icon in its colour. A kind this client predates shows a neutral mark. */
export function TodoKindIcon({ todo, className }: { todo: Pick<Todo, "kind" | "question_answer_type">; className?: string }) {
  const { Icon, className: tone } = isApproval(todo)
    ? { Icon: BadgeCheck, className: "text-brand" }
    : isKnownKind(todo.kind) ? KIND_ICONS[todo.kind] : { Icon: Info, className: "text-muted-foreground" };
  return <Icon aria-hidden className={cn("size-3.5 shrink-0", tone, className)} />;
}

/** A Quick create failed to-do whose run finished without a confirmed result: its issue may exist. */
export function isUnconfirmedQuickCreate(todo: Pick<Todo, "kind" | "details">): boolean {
  return todo.kind === "quick_create_failed" && todo.details?.reason === "unconfirmed";
}

/** The kind's label, such as "Question for you" or "Approval for you". */
export function useTodoKindLabel() {
  const { t } = useT("inbox");
  return useCallback(
    (todo: Pick<Todo, "kind" | "question_answer_type" | "details">): string => {
      if (isApproval(todo)) return t(($) => $.todos.kinds.approval);
      if (isUnconfirmedQuickCreate(todo)) return t(($) => $.types.quick_create_unconfirmed);
      return isKnownKind(todo.kind) ? t(($) => $.todos.kinds[todo.kind as TodoKind]) : t(($) => $.todos.kinds.other);
    },
    [t],
  );
}
