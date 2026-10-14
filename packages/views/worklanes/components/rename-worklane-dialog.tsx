"use client";

import { useEffect, useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Repository, Worklane, WorklaneRequestReason } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { repositoryListOptions } from "@multica/core/repositories";
import { useRenameWorklane, useWorklanes, worklaneRefusal } from "@multica/core/worklanes";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { useT } from "../../i18n";
import { useWorklaneReasonText } from "./worklane-reason";

/** Rename, opened with `{ worklaneId }` from a worklane's menu. */
export function RenameWorklaneDialog({
  onClose,
  data,
}: {
  onClose: () => void;
  data: Record<string, unknown> | null;
}) {
  const worklaneId = typeof data?.worklaneId === "string" ? data.worklaneId : "";
  const wsId = useWorkspaceId();
  const { data: worklanes, isPending } = useWorklanes(wsId);
  const { data: repositories } = useQuery(repositoryListOptions(wsId));
  const worklane = worklanes?.find((w) => w.id === worklaneId);
  const repository = repositories?.find((r) => r.id === worklane?.repository_id);

  // A worklane removed while the dialog opens closes it; main has no rename.
  const missing = !isPending && (!worklane || worklane.is_main);
  useEffect(() => {
    if (missing) onClose();
  }, [missing, onClose]);

  if (!worklane || worklane.is_main || !repository || !worklanes) return null;
  return (
    <RenameWorklaneForm
      worklane={worklane}
      repository={repository}
      siblings={worklanes.filter((w) => w.repository_id === worklane.repository_id && w.id !== worklane.id)}
      onClose={onClose}
    />
  );
}

function RenameWorklaneForm({
  worklane,
  repository,
  siblings,
  onClose,
}: {
  worklane: Worklane;
  repository: Repository;
  siblings: Worklane[];
  onClose: () => void;
}) {
  const { t } = useT("worklanes");
  const reasonText = useWorklaneReasonText();
  const renameWorklane = useRenameWorklane();
  const nameId = useId();
  const errorId = useId();
  const [name, setName] = useState(worklane.name);
  const [refusal, setRefusal] = useState<WorklaneRequestReason | null>(null);
  const [serverError, setServerError] = useState<string | null>(null);

  const trimmed = name.trim();
  const taken = siblings.some((w) => w.name.toLowerCase() === trimmed.toLowerCase());
  const nameError = taken
    ? t(($) => $.errors.name_taken, { repository: repository.full_name, name: trimmed })
    : trimmed === ""
      ? t(($) => $.errors.name_required)
      : null;
  const canSubmit = !nameError && trimmed !== worklane.name && !renameWorklane.isPending;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setRefusal(null);
    setServerError(null);
    try {
      await renameWorklane.mutateAsync({ id: worklane.id, name: trimmed });
      onClose();
    } catch (err) {
      const reason = worklaneRefusal(err);
      if (reason) setRefusal(reason);
      else setServerError(t(($) => $.rename.failed));
    }
  };

  const message =
    nameError ??
    (refusal
      ? reasonText(refusal, {
          repository: repository.full_name,
          name: trimmed,
          runtime: "",
          base: `origin/${repository.default_branch}`,
        })
      : serverError);

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t(($) => $.rename.title)}</DialogTitle>
          <DialogDescription>{t(($) => $.rename.description)}</DialogDescription>
        </DialogHeader>
        <form
          className="flex flex-col gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            void handleSubmit();
          }}
        >
          <Label htmlFor={nameId}>{t(($) => $.rename.name_label)}</Label>
          <Input
            id={nameId}
            autoFocus
            value={name}
            aria-invalid={message ? true : undefined}
            aria-describedby={message ? errorId : undefined}
            onChange={(e) => {
              setName(e.target.value);
              setRefusal(null);
              setServerError(null);
            }}
          />
          {message ? (
            <p id={errorId} role="alert" className="text-caption text-destructive">
              {message}
            </p>
          ) : null}
        </form>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            {t(($) => $.rename.cancel)}
          </Button>
          <Button
            type="button"
            disabled={!canSubmit}
            aria-busy={renameWorklane.isPending || undefined}
            onClick={() => void handleSubmit()}
          >
            {renameWorklane.isPending ? t(($) => $.rename.submitting) : t(($) => $.rename.submit)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
