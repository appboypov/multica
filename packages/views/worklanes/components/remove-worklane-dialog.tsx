"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Repository, Worklane, WorklaneRequestReason } from "@multica/core/types";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { repositoryListOptions } from "@multica/core/repositories";
import { runtimeListOptions } from "@multica/core/runtimes";
import {
  useRemoveWorklane,
  useWorklanes,
  worklaneRefusal,
  worklaneRequestOptions,
} from "@multica/core/worklanes";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { buildRuntimeMachines } from "../../runtimes/components/runtime-machines";
import { useWorklaneReasonText } from "./worklane-reason";

/** Remove, opened with `{ worklaneId }` from a worklane's menu. */
export function RemoveWorklaneDialog({
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
  // Held once found, so the dialog can say it is done after the worklane leaves the list.
  const [held, setHeld] = useState<{ worklane: Worklane; repository: Repository } | null>(null);
  useEffect(() => {
    if (worklane && repository && !held) setHeld({ worklane, repository });
  }, [worklane, repository, held]);

  const missing = !isPending && !held && (!worklane || worklane.is_main);
  useEffect(() => {
    if (missing) onClose();
  }, [missing, onClose]);

  if (!held || held.worklane.is_main) return null;
  return <RemoveWorklaneForm worklane={held.worklane} repository={held.repository} onClose={onClose} />;
}

/**
 * Names the folder Remove deletes on the machine, awaits the request on the
 * machine and shows a refusal's reason in place. Nothing is removed while the
 * worktree would lose work.
 */
function RemoveWorklaneForm({
  worklane,
  repository,
  onClose,
}: {
  worklane: Worklane;
  repository: Repository;
  onClose: () => void;
}) {
  const { t } = useT("worklanes");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const router = useNavigation();
  const reasonText = useWorklaneReasonText();
  const currentUserId = useAuthStore((s) => s.user?.id ?? null);
  const removeWorklane = useRemoveWorklane();
  const [requestId, setRequestId] = useState("");
  const [refusal, setRefusal] = useState<WorklaneRequestReason | null>(null);
  const [serverError, setServerError] = useState<string | null>(null);
  const { data: request } = useQuery(worklaneRequestOptions(wsId, requestId));

  const { data: runtimes = [] } = useQuery(runtimeListOptions(wsId));
  const runtimeName = useMemo(
    () =>
      buildRuntimeMachines(runtimes, { now: Date.now(), currentUserId }).find(
        (machine) => machine.daemonId === worklane.daemon_id,
      )?.title ?? "",
    [runtimes, currentUserId, worklane.daemon_id],
  );
  const base = `origin/${repository.default_branch}`;
  const running = removeWorklane.isPending || request?.status === "running";

  // A finished remove closes; on the worklane's own page it goes to its project.
  const done = request?.status === "completed";
  useEffect(() => {
    if (!done) return;
    onClose();
    if (router.pathname === paths.worklaneDetail(worklane.id)) {
      router.push(paths.projectDetail(repository.project_id));
    }
  }, [done, onClose, router, paths, worklane.id, repository.project_id]);

  // A refused or failed remove keeps the dialog with the reason.
  const endedReason =
    request && request.status !== "running" && request.status !== "completed" ? request.reason : null;
  useEffect(() => {
    if (endedReason) setRefusal(endedReason);
  }, [endedReason]);
  const endedWithoutReason =
    !!request && (request.status === "failed" || request.status === "unknown") && !request.reason;

  const handleRemove = async () => {
    if (running) return;
    setRefusal(null);
    setServerError(null);
    try {
      const started = await removeWorklane.mutateAsync(worklane.id);
      setRequestId(started.id);
    } catch (err) {
      const reason = worklaneRefusal(err);
      if (reason) setRefusal(reason);
      else setServerError(t(($) => $.remove.failed));
    }
  };

  const message = refusal
    ? reasonText(refusal, {
        repository: repository.full_name,
        name: worklane.name,
        runtime: runtimeName || request?.runtime_name || "",
        base,
      })
    : endedWithoutReason
      ? t(($) => $.remove.failed)
      : serverError;

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !running) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md" showCloseButton={!running}>
        <DialogHeader>
          <DialogTitle>{t(($) => $.remove.title, { name: worklane.name })}</DialogTitle>
          <DialogDescription>
            {t(($) => $.remove.description, {
              path: worklane.path ?? worklane.folder ?? "",
              runtime: runtimeName || t(($) => $.remove.unknown_runtime),
              base,
            })}
          </DialogDescription>
        </DialogHeader>
        {message ? (
          <p role="alert" className="text-caption text-destructive">
            {message}
          </p>
        ) : null}
        <DialogFooter>
          <Button type="button" variant="outline" disabled={running} onClick={onClose}>
            {t(($) => $.remove.cancel)}
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={running}
            aria-busy={running || undefined}
            onClick={() => void handleRemove()}
          >
            {running ? t(($) => $.remove.submitting) : t(($) => $.remove.submit)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
