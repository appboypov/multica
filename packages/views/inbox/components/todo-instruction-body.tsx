"use client";

import { useAnswerCheckTodo } from "@multica/core/todos";
import type { Todo } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { checkWorkLink } from "./check-work-link";

export function TodoInstructionBody({ todo }: { todo: Todo }) {
  const { t } = useT("inbox");
  const answer = useAnswerCheckTodo();
  const text = (key: string) => typeof todo.details[key] === "string" ? todo.details[key] as string : "";
  const commit = checkWorkLink(todo.details.link);
  const undo = checkWorkLink(todo.details.undo_link);
  const requested = todo.details.undo_requested === true;
  return <section className="flex flex-col gap-4" aria-label={t(($) => $.checks.instruction_change)}>
    <h3 className="text-body font-medium">{text("file")}</h3>
    <p className="text-caption text-muted-foreground">{t(($) => $.checks.linked_check, { id: text("check_id") })}</p>
    <p className="whitespace-pre-wrap text-body">{text("note")}</p>
    {commit && <a href={commit} target="_blank" rel="noopener noreferrer" className="text-brand underline">{t(($) => $.checks.open_commit)}</a>}
    {requested && <p className="text-body text-muted-foreground">{text("undo_note") || t(($) => $.checks.undo_waiting)}</p>}
    {undo && <a href={undo} target="_blank" rel="noopener noreferrer" className="text-brand underline">{t(($) => $.checks.open_undo)}</a>}
    {answer.isError && <p role="alert" className="text-destructive">{t(($) => $.checks.answer_failed)}</p>}
    <div className="flex flex-wrap gap-2" aria-busy={answer.isPending}>
      <Button type="button" disabled={answer.isPending} onClick={() => answer.mutate({ id: todo.id, answer: "keep", note: "" })}>{t(($) => $.checks.keep)}</Button>
      <Button type="button" variant="outline" disabled={answer.isPending || requested} onClick={() => answer.mutate({ id: todo.id, answer: "undo", note: "" })}>{t(($) => $.checks.undo)}</Button>
    </div>
  </section>;
}
