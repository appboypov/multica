"use client";

import { useEffect, useId, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { Repository, WorklaneRequestReason } from "@multica/core/types";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { organisationDetailOptions } from "@multica/core/organisations";
import { useWorkspacePaths } from "@multica/core/paths";
import { projectDetailOptions } from "@multica/core/projects/queries";
import {
  repositoryCheckoutOptions,
  repositoryKeys,
  repositoryListOptions,
} from "@multica/core/repositories";
import { runtimeAdvertisesWorklaneWorktrees, runtimeListOptions } from "@multica/core/runtimes";
import {
  folderFromName,
  folderProblem,
  useCreateWorklaneRequest,
  useRetryWorklaneRequest,
  useWorklanes,
  worklanePath,
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
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { Spinner } from "@multica/ui/components/ui/spinner";
import { useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { buildRuntimeMachines } from "../../runtimes/components/runtime-machines";
import { useWorklaneReasonText } from "./worklane-reason";
import { WorklaneRequestSteps } from "./worklane-request-steps";

/**
 * New worklane, opened with `{ repositoryId }` from a repository's card or
 * its row on Projects and an organisation's page.
 */
export function NewWorklaneDialog({
  onClose,
  data,
}: {
  onClose: () => void;
  data: Record<string, unknown> | null;
}) {
  const repositoryId = typeof data?.repositoryId === "string" ? data.repositoryId : "";
  const wsId = useWorkspaceId();
  const { data: repositories, isPending } = useQuery(repositoryListOptions(wsId));
  const repository = repositories?.find((r) => r.id === repositoryId);

  // A repository removed while the dialog opens closes it.
  const missing = !isPending && !repository;
  useEffect(() => {
    if (missing) onClose();
  }, [missing, onClose]);

  if (!repository) return null;
  return <NewWorklaneForm repository={repository} onClose={onClose} />;
}

type MachineReason = "no_checkout" | "offline" | "cannot_make_worktrees";

function NewWorklaneForm({
  repository,
  onClose,
}: {
  repository: Repository;
  onClose: () => void;
}) {
  const { t } = useT("worklanes");
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const paths = useWorkspacePaths();
  const router = useNavigation();
  const reasonText = useWorklaneReasonText();
  const currentUserId = useAuthStore((s) => s.user?.id ?? null);
  const nameId = useId();
  const folderId = useId();
  const folderHintId = useId();
  const nameErrorId = useId();

  const [name, setName] = useState("");
  const [nameTouched, setNameTouched] = useState(false);
  const [folderDraft, setFolderDraft] = useState<string | null>(null);
  const [daemonId, setDaemonId] = useState("");
  const [requestId, setRequestId] = useState("");
  const [refusal, setRefusal] = useState<WorklaneRequestReason | null>(null);
  const [serverError, setServerError] = useState<string | null>(null);

  const createRequest = useCreateWorklaneRequest();
  const retryRequest = useRetryWorklaneRequest();
  const { data: request } = useQuery(worklaneRequestOptions(wsId, requestId));

  const { data: allWorklanes = [] } = useWorklanes(wsId);
  const worklanes = useMemo(
    () => allWorklanes.filter((worklane) => worklane.repository_id === repository.id),
    [allWorklanes, repository.id],
  );

  const { data: project } = useQuery(projectDetailOptions(wsId, repository.project_id));
  const organisationId = project?.organisation_id ?? null;
  const { data: organisation } = useQuery({
    ...organisationDetailOptions(wsId, organisationId ?? ""),
    enabled: !!organisationId,
  });

  const { data: runtimes = [] } = useQuery(runtimeListOptions(wsId));
  const machines = useMemo(
    () =>
      buildRuntimeMachines(runtimes, { now: Date.now(), currentUserId }).filter(
        (machine) => machine.daemonId,
      ),
    [runtimes, currentUserId],
  );
  const reasonOf = (machine: (typeof machines)[number]): MachineReason | null => {
    if (machine.daemonId !== repository.checkout_daemon_id) return "no_checkout";
    if (machine.onlineCount === 0) return "offline";
    if (!runtimeAdvertisesWorklaneWorktrees(runtimes, machine.daemonId)) return "cannot_make_worktrees";
    return null;
  };
  const checkoutMachine = machines.find((m) => m.daemonId === repository.checkout_daemon_id) ?? null;
  const checkoutPickable = !!checkoutMachine && reasonOf(checkoutMachine) === null;
  const machine = machines.find((m) => m.daemonId === daemonId) ?? null;
  const machinePickable = !!machine && reasonOf(machine) === null;

  // Runtime fills in with the checkout's machine while it can be picked.
  useEffect(() => {
    if (!daemonId && checkoutPickable && checkoutMachine?.daemonId) {
      setDaemonId(checkoutMachine.daemonId);
    }
  }, [daemonId, checkoutPickable, checkoutMachine]);

  // The install command is found by a check of the checkout; the server
  // saves it and publishes `repository:updated`.
  const installKnown = repository.install_command !== null;
  const checkoutRuntime = checkoutMachine?.runtimes.find((r) => r.status === "online") ?? null;
  const installCheck = useQuery({
    ...repositoryCheckoutOptions(
      checkoutRuntime?.id ?? "",
      repository.full_name,
      repository.default_branch,
      repository.checkout_path,
    ),
    enabled: !installKnown && !!checkoutRuntime,
  });
  const installCheckDone = installCheck.isSuccess;
  useEffect(() => {
    if (installCheckDone) void qc.invalidateQueries({ queryKey: repositoryKeys.list(wsId) });
  }, [installCheckDone, qc, wsId]);

  const trimmedName = name.trim();
  const folder = folderDraft ?? folderFromName(name);
  const nameTaken = worklanes.find(
    (worklane) => worklane.name.toLowerCase() === trimmedName.toLowerCase(),
  );
  const nameError = nameTaken
    ? t(($) => $.new.name_taken, { repository: repository.full_name, name: trimmedName })
    : nameTouched && trimmedName === ""
      ? t(($) => $.errors.name_required)
      : null;
  const folderIssue = folderProblem(folder, worklanes);
  const showFolderIssue = folderIssue && (folder !== "" || folderDraft !== null);
  const base = `origin/${repository.default_branch}`;
  const path = worklanePath(organisation?.key ?? null, repository.full_name, folder || "…");

  const running = request?.status === "running" || createRequest.isPending || retryRequest.isPending;
  const failed = request?.status === "failed" || request?.status === "unknown";
  const canCreate =
    trimmedName !== "" &&
    !nameTaken &&
    !folderIssue &&
    machinePickable &&
    installKnown &&
    !running;

  // A finished request opens the worklane; a refused one goes back to the fields.
  const completedWorklaneId = request?.status === "completed" ? request.worklane_id : null;
  useEffect(() => {
    if (!completedWorklaneId) return;
    onClose();
    router.push(paths.worklaneDetail(completedWorklaneId));
  }, [completedWorklaneId, onClose, router, paths]);
  const refusedReason = request?.status === "refused" ? request.reason : null;
  useEffect(() => {
    if (!refusedReason) return;
    setRefusal(refusedReason);
    setRequestId("");
  }, [refusedReason]);

  const reasonContext = {
    repository: repository.full_name,
    name: trimmedName,
    runtime: machine?.title ?? "",
    base,
  };

  const handleFailure = (err: unknown) => {
    const reason = worklaneRefusal(err);
    if (reason) setRefusal(reason);
    else setServerError(t(($) => $.new.failed));
  };

  const handleCreate = async () => {
    if (!canCreate || !daemonId) return;
    setRefusal(null);
    setServerError(null);
    try {
      const created = await createRequest.mutateAsync({
        repository_id: repository.id,
        name: trimmedName,
        folder,
        daemon_id: daemonId,
      });
      setRequestId(created.id);
    } catch (err) {
      handleFailure(err);
    }
  };

  const handleRetry = async () => {
    if (!request) return;
    setServerError(null);
    try {
      const retried = await retryRequest.mutateAsync(request.id);
      setRequestId(retried.id);
    } catch (err) {
      handleFailure(err);
    }
  };

  const machineLabel = (m: (typeof machines)[number]) =>
    m.onlineCount > 0
      ? t(($) => $.new.runtime_online, { name: m.title })
      : t(($) => $.new.runtime_offline, { name: m.title });
  const machineReasonLabel = (reason: MachineReason) =>
    reason === "no_checkout"
      ? t(($) => $.new.runtime_reason.no_checkout, { repository: repository.full_name })
      : t(($) => $.new.runtime_reason[reason]);

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !running) onClose();
      }}
    >
      <DialogContent className="sm:max-w-lg" showCloseButton={!running}>
        <DialogHeader>
          <DialogTitle>{t(($) => $.new.title, { repository: repository.full_name })}</DialogTitle>
          {request ? null : <DialogDescription>{t(($) => $.new.description)}</DialogDescription>}
        </DialogHeader>

        {request ? (
          <div className="flex flex-col gap-4">
            <div className="space-y-0.5">
              <p className="text-body font-medium text-foreground">{request.name}</p>
              <p className="truncate font-mono text-caption text-muted-foreground">{request.path}</p>
              <p className="text-caption text-muted-foreground">
                {t(($) => $.new.creating_on, { runtime: request.runtime_name })}
              </p>
            </div>
            <WorklaneRequestSteps request={request} />
            {failed ? (
              <div role="alert" className="space-y-1">
                {request.reason ? (
                  <p className="text-caption text-destructive">
                    {reasonText(request.reason, { ...reasonContext, name: request.name, runtime: request.runtime_name })}
                  </p>
                ) : null}
                <p className="text-caption text-muted-foreground">{t(($) => $.new.failed_note)}</p>
              </div>
            ) : null}
            {serverError ? (
              <p role="alert" className="text-caption text-destructive">{serverError}</p>
            ) : null}
          </div>
        ) : (
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor={nameId}>{t(($) => $.new.name_label)}</Label>
              <Input
                id={nameId}
                autoFocus
                value={name}
                placeholder={t(($) => $.new.name_placeholder)}
                aria-invalid={nameError ? true : undefined}
                aria-describedby={nameError ? nameErrorId : undefined}
                onChange={(e) => {
                  setName(e.target.value);
                  setRefusal(null);
                }}
                onBlur={() => setNameTouched(true)}
              />
              {nameError ? (
                <p id={nameErrorId} role="alert" className="text-caption text-destructive">
                  {nameError}
                </p>
              ) : null}
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor={folderId}>{t(($) => $.new.folder_label)}</Label>
              <Input
                id={folderId}
                className="font-mono"
                value={folder}
                aria-invalid={showFolderIssue ? true : undefined}
                aria-describedby={folderHintId}
                onChange={(e) => {
                  setFolderDraft(e.target.value);
                  setRefusal(null);
                }}
              />
              <p
                id={folderHintId}
                className={showFolderIssue ? "text-caption text-destructive" : "text-caption text-muted-foreground"}
              >
                {showFolderIssue && folderIssue
                  ? folderIssue.kind === "invalid"
                    ? t(($) => $.new.folder_invalid)
                    : t(($) => $.new.folder_in_use, { name: folderIssue.worklaneName })
                  : t(($) => $.new.folder_hint, { repository: repository.full_name })}
              </p>
            </div>

            <div className="flex flex-col gap-2">
              <Label>{t(($) => $.new.runtime_label)}</Label>
              <Select
                items={machines.map((m) => ({ value: m.daemonId as string, label: m.title }))}
                value={machinePickable ? daemonId : null}
                onValueChange={(value) => setDaemonId((value as string | null) ?? "")}
              >
                <SelectTrigger aria-label={t(($) => $.new.runtime_label)} className="w-full">
                  <SelectValue placeholder={t(($) => $.new.runtime_placeholder)}>
                    {() =>
                      machinePickable && machine
                        ? machineLabel(machine)
                        : checkoutMachine
                          ? machineLabel(checkoutMachine)
                          : t(($) => $.new.runtime_placeholder)
                    }
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {machines.map((m) => {
                    const reason = reasonOf(m);
                    return (
                      <SelectItem key={m.id} value={m.daemonId as string} disabled={reason !== null}>
                        <span className="flex min-w-0 flex-1 items-baseline justify-between gap-3">
                          <span className="truncate">{machineLabel(m)}</span>
                          {reason ? (
                            <span className="shrink-0 text-caption text-muted-foreground">
                              {machineReasonLabel(reason)}
                            </span>
                          ) : null}
                        </span>
                      </SelectItem>
                    );
                  })}
                </SelectContent>
              </Select>
              <p className="text-caption text-muted-foreground">{t(($) => $.new.runtime_hint)}</p>
            </div>

            <div className="flex flex-col gap-2">
              <Label>{t(($) => $.new.worktree_label)}</Label>
              <div className="space-y-1 rounded-lg border bg-muted/40 px-3 py-2">
                <p className="truncate font-mono text-body text-foreground">{path}</p>
                <p className="text-caption text-muted-foreground">
                  {t(($) => $.new.worktree_base, { base })}
                </p>
                {installKnown ? (
                  <p className="truncate font-mono text-caption text-muted-foreground">
                    {repository.install_command === ""
                      ? t(($) => $.new.install_none)
                      : repository.install_command}
                  </p>
                ) : (
                  <p className="flex items-center gap-1.5 text-caption text-muted-foreground">
                    <Spinner className="size-3" />
                    {t(($) => $.new.finding_install)}
                  </p>
                )}
              </div>
            </div>

            {refusal ? (
              <p role="alert" className="text-caption text-destructive">
                {reasonText(refusal, reasonContext)}
              </p>
            ) : null}
            {serverError ? (
              <p role="alert" className="text-caption text-destructive">{serverError}</p>
            ) : null}
          </div>
        )}

        <DialogFooter>
          <Button type="button" variant="outline" disabled={running} onClick={onClose}>
            {t(($) => $.new.cancel)}
          </Button>
          {failed && !running ? (
            <Button type="button" onClick={() => void handleRetry()}>
              {t(($) => $.new.try_again)}
            </Button>
          ) : (
            <Button
              type="button"
              disabled={!canCreate}
              aria-busy={running || undefined}
              onClick={() => void handleCreate()}
            >
              {running ? t(($) => $.new.creating) : t(($) => $.new.create)}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
