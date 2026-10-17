"use client";

import { useState } from "react";
import { useAnswerCheckTodo } from "@multica/core/todos";
import { isCheckKind, type Todo } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";
import { checkWorkLink } from "./check-work-link";
import { ProjectCheckRecord } from "./project-check-record";

export function TodoCheckBody({ todo }: { todo: Todo }) {
  const { t } = useT("inbox");
  const [note, setNote] = useState("");
  const answer = useAnswerCheckTodo();
  const projectId = typeof todo.details.project_id === "string" ? todo.details.project_id : "";
  const kind = isCheckKind(todo.details.check_kind) ? todo.details.check_kind : null;
  const link = checkWorkLink(todo.details.link);
  return <section className="flex flex-col gap-5" aria-label={t(($) => $.checks.detail)}>
    <div className="flex flex-col gap-2">
      <h3 className="text-body font-medium">{kind ? t(($) => $.checks.kind_labels[kind]) : t(($) => $.todos.kinds.check)}</h3>
      <p className="text-body text-muted-foreground">{kind === "release" ? t(($) => $.checks.before_release) : kind && ["data_model", "data_flow", "design", "prototype"].includes(kind) ? t(($) => $.checks.before_work) : t(($) => $.checks.on_staging)}</p>
      {todo.details.sampled === true && <p className="text-caption text-muted-foreground">{t(($) => $.checks.sampled)}</p>}
      {link && <a href={link} target="_blank" rel="noopener noreferrer" className="text-body text-brand underline">{t(($) => $.checks.open_work)}</a>}
    </div>
    <label htmlFor={`check-note-${todo.id}`} className="text-body font-medium">{t(($) => $.checks.feedback_label)}</label>
    <Textarea id={`check-note-${todo.id}`} value={note} onChange={(event) => setNote(event.target.value)} disabled={answer.isPending} rows={4} />
    {answer.isError && <p role="alert" className="text-body text-destructive">{t(($) => $.checks.answer_failed)}</p>}
    <div className="flex flex-wrap items-center gap-2" aria-busy={answer.isPending}>
      <Button type="button" disabled={!kind || answer.isPending} onClick={() => answer.mutate({ id: todo.id, answer: "pass", note })}>{t(($) => $.checks.pass)}</Button>
      <Button type="button" variant="outline" disabled={!kind || answer.isPending || !note.trim()} onClick={() => answer.mutate({ id: todo.id, answer: "feedback", note })}>{t(($) => $.checks.send_feedback)}</Button>
    </div>
    {projectId && <ProjectCheckRecord projectId={projectId} />}
  </section>;
}
