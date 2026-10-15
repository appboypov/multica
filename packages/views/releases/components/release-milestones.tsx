"use client";

import { useId, useState } from "react";
import { Pencil, Plus, Trash2, Loader2 } from "lucide-react";
import { clientErrorMessage } from "@multica/core/api";
import { useAddReleaseMilestone, useEditReleaseMilestone, useRemoveReleaseMilestone } from "@multica/core/releases";
import type { Release, ReleaseMilestone } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { useT } from "../../i18n";

/** Milestones save independently from the release's version and title. */
export function ReleaseMilestones({ release }: { release: Release }) {
  const { t } = useT("releases");
  const nameId = useId();
  const dateId = useId();
  const add = useAddReleaseMilestone();
  const edit = useEditReleaseMilestone();
  const remove = useRemoveReleaseMilestone();
  const [editing, setEditing] = useState<string | null>(null);
  const [removing, setRemoving] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [date, setDate] = useState("");
  const [error, setError] = useState<string | null>(null);
  const pending = add.isPending || edit.isPending || remove.isPending;

  const start = (milestone?: ReleaseMilestone) => {
    setEditing(milestone?.id ?? "new");
    setRemoving(null);
    setName(milestone?.name ?? "");
    setDate(milestone?.date ?? "");
    setError(null);
  };
  const save = async () => {
    if (!name.trim() || !date || pending) return;
    setError(null);
    try {
      const data = { name: name.trim(), date };
      if (editing === "new") {
        await add.mutateAsync({ releaseId: release.id, repositoryId: release.repository_id, data });
      } else if (editing) {
        await edit.mutateAsync({ id: editing, repositoryId: release.repository_id, data });
      }
      setEditing(null);
    } catch (err) {
      setError(clientErrorMessage(err) ?? t(($) => $.milestones.failed));
    }
  };
  const confirmRemove = async (id: string) => {
    setError(null);
    try {
      await remove.mutateAsync({ id, repositoryId: release.repository_id });
      setRemoving(null);
    } catch (err) {
      setError(clientErrorMessage(err) ?? t(($) => $.milestones.failed));
    }
  };

  return (
    <section aria-label={t(($) => $.milestones.title)} className="min-w-0 space-y-2">
      <div className="flex items-center justify-between gap-2">
        <h4 className="text-label font-medium">{t(($) => $.milestones.title)}</h4>
        <Button type="button" variant="ghost" size="xs" disabled={pending} onClick={() => start()}>
          <Plus aria-hidden="true" />
          {t(($) => $.milestones.add)}
        </Button>
      </div>
      {release.milestones.length > 0 ? (
        <ul className="space-y-1">
          {release.milestones.map((milestone) => (
            <li key={milestone.id} className="min-w-0">
              <div className="flex items-start gap-2 text-label">
                <span className="min-w-0 flex-1 break-words">{milestone.name}</span>
                <time dateTime={milestone.date} className="shrink-0 pt-0.5 text-caption tabular-nums text-muted-foreground">
                  {milestone.date}
                </time>
                <Button type="button" variant="ghost" size="icon-xs" disabled={pending} aria-label={t(($) => $.milestones.edit_name, { name: milestone.name })} onClick={() => start(milestone)}>
                  <Pencil aria-hidden="true" />
                </Button>
                <Button type="button" variant="ghost" size="icon-xs" disabled={pending} aria-label={t(($) => $.milestones.remove_name, { name: milestone.name })} onClick={() => { setRemoving(milestone.id); setEditing(null); setError(null); }}>
                  <Trash2 aria-hidden="true" />
                </Button>
              </div>
              {removing === milestone.id ? (
                <div className="mt-1 space-y-2">
                  <p className="text-caption">{t(($) => $.milestones.confirm_remove, { name: milestone.name })}</p>
                  <div className="flex justify-end gap-2">
                    <Button type="button" variant="outline" size="xs" disabled={pending} onClick={() => setRemoving(null)}>{t(($) => $.milestones.cancel)}</Button>
                    <Button type="button" variant="destructive" size="xs" disabled={pending} aria-busy={remove.isPending} onClick={() => void confirmRemove(milestone.id)}>
                      {remove.isPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : null}
                      {t(($) => $.milestones.remove)}
                    </Button>
                  </div>
                </div>
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}
      {editing !== null ? (
        <form className="space-y-2" onSubmit={(event) => { event.preventDefault(); void save(); }}>
          <div className="space-y-1">
            <Label htmlFor={nameId}>{t(($) => $.milestones.name)}</Label>
            <Input id={nameId} required autoFocus value={name} disabled={pending} onChange={(event) => setName(event.target.value)} />
          </div>
          <div className="space-y-1">
            <Label htmlFor={dateId}>{t(($) => $.milestones.date)}</Label>
            <Input id={dateId} type="date" required value={date} disabled={pending} onChange={(event) => setDate(event.target.value)} />
          </div>
          <div className="flex items-center justify-end gap-2">
            <Button type="button" variant="outline" size="xs" disabled={pending} onClick={() => setEditing(null)}>{t(($) => $.milestones.cancel)}</Button>
            <Button type="submit" size="xs" disabled={pending || !name.trim() || !date} aria-busy={add.isPending || edit.isPending}>
              {add.isPending || edit.isPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : null}
              {t(($) => $.milestones.save)}
            </Button>
          </div>
          <p className="text-caption text-muted-foreground">{t(($) => $.milestones.independent)}</p>
        </form>
      ) : null}
      {error ? <p role="alert" className="text-caption text-destructive">{error}</p> : null}
    </section>
  );
}
