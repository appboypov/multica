"use client";

import { useState } from "react";
import { ArrowUpRight, LoaderCircle } from "lucide-react";
import { useAnswerCheckTodo, useProjectCheckTrust } from "@multica/core/todos";
import type { Todo } from "@multica/core/types";
import { isCheckKind } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";

function workLink(value: unknown): string | null {
  if (typeof value !== "string") return null;
  try { const url = new URL(value); return url.protocol === "https:" || url.protocol === "http:" ? url.href : null; } catch { return null; }
}

export function TodoCheckBody({ todo }: { todo: Todo }) {
  const { t } = useT("inbox");
  const [note, setNote] = useState("");
  const answer = useAnswerCheckTodo();
  const projectId = typeof todo.details.project_id === "string" ? todo.details.project_id : "";
  const trust = useProjectCheckTrust(projectId);
  const kind = isCheckKind(todo.details.check_kind) ? todo.details.check_kind : null;
  const proposal = todo.kind === "trust_proposal";
  const link = workLink(todo.details.link);
  const pass = () => answer.mutate({ id: todo.id, answer: proposal ? "yes" : "pass", note });
  const feedback = () => answer.mutate({ id: todo.id, answer: proposal ? "no" : "feedback", note });
  const count = (key: string): number => {
    const value = todo.details[key];
    return typeof value === "number" ? value : 0;
  };
  const kindLabel = (value: unknown) => isCheckKind(value) ? t(($) => $.checks.kind_labels[value]) : t(($) => $.todos.kinds.check);

  return (
    <section className="flex flex-col gap-5" aria-label={t(($) => $.checks.detail)}>
      <div className="flex flex-col gap-2">
        <h3 className="text-body font-medium">{kindLabel(kind)}</h3>
        {proposal ? (
          <p className="text-body text-muted-foreground">{t(($) => $.checks.proposal_record, { passed: count("passed"), feedback: count("feedback"), streak: count("streak") })}</p>
        ) : (
          <p className="text-body text-muted-foreground">{kind === "release" ? t(($) => $.checks.before_release) : kind && ["data_model", "data_flow", "design", "prototype"].includes(kind) ? t(($) => $.checks.before_work) : t(($) => $.checks.on_staging)}</p>
        )}
        {todo.details.sampled === true && <p className="text-caption text-muted-foreground">{t(($) => $.checks.sampled)}</p>}
        {link && <a href={link} target="_blank" rel="noopener noreferrer" className="inline-flex w-fit items-center gap-1 text-body text-brand underline underline-offset-4 break-all">{t(($) => $.checks.open_work)}<ArrowUpRight className="size-3.5 shrink-0" /></a>}
      </div>
      {proposal && <p className="text-body text-muted-foreground">{t(($) => $.checks.proposal_effect)}</p>}
      {!proposal && (
        <div className="flex flex-col gap-2">
          <label htmlFor={`check-note-${todo.id}`} className="text-body font-medium">{t(($) => $.checks.feedback_label)}</label>
          <Textarea id={`check-note-${todo.id}`} value={note} onChange={(event) => setNote(event.target.value)} disabled={answer.isPending} rows={4} />
        </div>
      )}
      {answer.isError && <p role="alert" className="text-body text-destructive">{t(($) => $.checks.answer_failed)}</p>}
      <div className="flex flex-wrap items-center gap-2" aria-busy={answer.isPending}>
        <Button type="button" onClick={pass} disabled={!kind || answer.isPending}>
          {answer.isPending && <LoaderCircle className="animate-spin" />}
          {proposal ? t(($) => $.checks.yes) : t(($) => $.checks.pass)}
        </Button>
        <Button type="button" variant="outline" onClick={feedback} disabled={!kind || answer.isPending || (!proposal && !note.trim())}>
          {proposal ? t(($) => $.checks.no) : t(($) => $.checks.send_feedback)}
        </Button>
      </div>
      <details className="text-body">
        <summary className="w-fit cursor-pointer font-medium">{t(($) => $.checks.project_trust)}</summary>
        {trust.isPending && <p className="mt-3 text-muted-foreground">{t(($) => $.checks.loading)}</p>}
        {trust.isError && <div className="mt-3 flex flex-wrap items-center gap-2"><p role="alert" className="text-destructive">{t(($) => $.checks.trust_failed)}</p><Button type="button" variant="outline" size="sm" onClick={() => void trust.refetch()}>{t(($) => $.checks.retry)}</Button></div>}
        {trust.data && <div className="mt-3 overflow-x-auto"><table className="w-full text-left text-caption">
          <thead className="text-muted-foreground"><tr><th className="py-2 pr-4">{t(($) => $.checks.kind)}</th><th className="py-2 pr-4">{t(($) => $.checks.mode)}</th><th className="py-2 pr-4">{t(($) => $.checks.passed)}</th><th className="py-2 pr-4">{t(($) => $.checks.feedback_label)}</th><th className="py-2">{t(($) => $.checks.automatic_passed)}</th></tr></thead>
          <tbody>{trust.data.map((state) => <tr key={state.kind} className="border-t"><td className="py-2 pr-4">{kindLabel(state.kind)}</td><td className="py-2 pr-4">{state.automatic ? t(($) => $.checks.automatic) : t(($) => $.checks.checked)}</td><td className="py-2 pr-4 tabular-nums">{state.passed}</td><td className="py-2 pr-4 tabular-nums">{state.feedback}</td><td className="py-2 tabular-nums">{state.automatic_passed}</td></tr>)}</tbody>
        </table></div>}
      </details>
    </section>
  );
}
