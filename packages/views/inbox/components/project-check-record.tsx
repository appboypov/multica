"use client";

import { useState } from "react";
import { useCheckHistory, useProjectCheckTrust, useSetCheckSampling } from "@multica/core/todos";
import { isCheckKind, type CheckTrust } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { checkWorkLink } from "./check-work-link";


function CheckRecordRow({ state }: { state: CheckTrust }) {
  const { t } = useT("inbox");
  const [expanded, setExpanded] = useState(false);
  const history = useCheckHistory(state.project_id, state.kind, expanded);
  const mode = useSetCheckSampling();
  const kind = isCheckKind(state.kind) ? state.kind : null;
  return <section className="flex flex-col gap-2 border-t py-3">
    <h4 className="text-body font-medium">{kind ? t(($) => $.checks.kind_labels[kind]) : state.kind}</h4>
    <p className="text-caption text-muted-foreground">{state.sampling ? t(($) => $.checks.samples) : t(($) => $.checks.checked)}</p>
    <p className="text-caption">{t(($) => $.checks.record_counts, { passed: state.passed, feedback: state.feedback, automatic: state.automatic_passed })}</p>
    {state.can_manage && kind && kind !== "release" && <Button type="button" size="sm" variant="outline" disabled={mode.isPending} onClick={() => mode.mutate({ projectId: state.project_id, kind, sampling: !state.sampling })}>{state.sampling ? t(($) => $.checks.check_every_piece) : t(($) => $.checks.use_samples)}</Button>}
    {mode.isError && <p role="alert" className="text-caption text-destructive">{t(($) => $.checks.mode_failed)}</p>}
    <Button type="button" size="sm" variant="ghost" aria-expanded={expanded} onClick={() => setExpanded(!expanded)}>{t(($) => $.checks.history)}</Button>
    {expanded && <div className="flex flex-col gap-2">
      {history.isPending && <p>{t(($) => $.checks.loading)}</p>}
      {history.isError && <div role="alert"><p>{t(($) => $.checks.history_failed)}</p><Button type="button" variant="outline" size="sm" onClick={() => void history.refetch()}>{t(($) => $.checks.retry)}</Button></div>}
      {history.data?.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.checks.no_history)}</p>}
      {history.data?.map((item) => {
        const link = checkWorkLink(item.link);
        return <article key={item.id} className="border-l pl-3 text-caption">
          <p className="font-medium">{item.title}</p>
          <p>{item.answer === "feedback" ? t(($) => $.checks.feedback_label) : item.automatic ? t(($) => $.checks.automatic_passed) : t(($) => $.checks.passed)}{item.sampled ? ` · ${t(($) => $.checks.sampled)}` : ""}</p>
          {item.note && <p className="whitespace-pre-wrap">{item.note}</p>}
          <time dateTime={item.answered_at}>{new Date(item.answered_at).toLocaleString()}</time>
          {link && <a href={link} target="_blank" rel="noopener noreferrer" className="ml-2 text-brand underline">{t(($) => $.checks.open_work)}</a>}
        </article>;
      })}
    </div>}
  </section>;
}

export function ProjectCheckRecord({ projectId }: { projectId: string }) {
  const { t } = useT("inbox");
  const record = useProjectCheckTrust(projectId);
  return <section className="flex flex-col gap-3" aria-label={t(($) => $.checks.project_trust)}>
    <h3 className="text-body font-medium">{t(($) => $.checks.project_trust)}</h3>
    {record.isPending && <p>{t(($) => $.checks.loading)}</p>}
    {record.isError && <div role="alert"><p>{t(($) => $.checks.trust_failed)}</p><Button type="button" variant="outline" size="sm" onClick={() => void record.refetch()}>{t(($) => $.checks.retry)}</Button></div>}
    {record.data?.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.checks.no_history)}</p>}
    {record.data?.map((state) => <CheckRecordRow key={state.kind} state={state} />)}
  </section>;
}
