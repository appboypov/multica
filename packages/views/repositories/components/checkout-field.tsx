"use client";

import { useEffect, useId, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CircleAlert, CircleCheck, FolderOpen } from "lucide-react";
import type { RepositoryCheckoutFailure, RepositoryCheckoutRequest } from "@multica/core/types";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { createLogger } from "@multica/core/logger";
import { runtimeListOptions } from "@multica/core/runtimes";
import {
  repositoryCheckoutKeys,
  repositoryCheckoutOptions,
  resolveRepositoryCheckout,
} from "@multica/core/repositories";
import { Button } from "@multica/ui/components/ui/button";
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
import { useT } from "../../i18n";
import { githubShortLabel } from "../../common/github-url";
import { RuntimeFileBrowser } from "../../agents/components/runtime-file-browser";
import { buildRuntimeMachines } from "../../runtimes/components/runtime-machines";

const logger = createLogger("repositories.checkout");

/** A checkout: the machine by its daemon id and the path on it. */
export interface CheckoutValue {
  daemonId: string;
  path: string;
}

/**
 * Checkout: a machine of the viewer and the path of the repository's normal
 * checkout on it. The check runs on the machine whenever the repository, the
 * machine or the path changes, and `onCheck` gets the completed check of the
 * current value, or null while it runs or could not run. A missing folder
 * offers Clone it there, which clones and then checks.
 */
export function CheckoutField({
  fullName,
  defaultBranch,
  value,
  onChange,
  onCheck,
  disabled,
}: {
  /** The picked repository; null disables the field. */
  fullName: string | null;
  defaultBranch: string | null;
  value: CheckoutValue;
  onChange: (value: CheckoutValue) => void;
  onCheck: (check: RepositoryCheckoutRequest | null) => void;
  disabled?: boolean;
}) {
  const { t } = useT("repositories");
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const currentUserId = useAuthStore((s) => s.user?.id ?? null);
  const pathId = useId();
  const [browseOpen, setBrowseOpen] = useState(false);
  const [draftPath, setDraftPath] = useState<string | null>(null);

  // Only the runtime's owner may read its disk (ADR-0001), so only the
  // viewer's own local runtimes can hold a checkout.
  const { data: runtimes = [] } = useQuery(runtimeListOptions(wsId, "me"));
  const machines = useMemo(
    () =>
      buildRuntimeMachines(
        runtimes.filter((runtime) => runtime.runtime_mode === "local" && runtime.daemon_id),
        { now: Date.now(), currentUserId },
      ),
    [runtimes, currentUserId],
  );
  const machine = machines.find((m) => m.daemonId === value.daemonId) ?? null;
  const runtime = machine?.runtimes.find((r) => r.status === "online") ?? null;

  // Fill in the first online machine, the viewer's own first.
  useEffect(() => {
    if (value.daemonId) return;
    const first = machines.find((m) => m.daemonId && m.onlineCount > 0);
    if (first?.daemonId) onChange({ ...value, daemonId: first.daemonId });
  }, [machines, value, onChange]);

  const ready = !!fullName && !!defaultBranch && !!runtime && value.path.trim() !== "";
  const check = useQuery({
    ...repositoryCheckoutOptions(
      runtime?.id ?? "",
      fullName ?? "",
      defaultBranch ?? "",
      value.path.trim(),
    ),
    enabled: ready && !disabled,
  });

  const clone = useMutation({
    mutationFn: () =>
      resolveRepositoryCheckout(runtime!.id, {
        full_name: fullName!,
        default_branch: defaultBranch!,
        path: value.path.trim(),
        clone: true,
      }),
    onSuccess: (result) =>
      qc.setQueryData(
        repositoryCheckoutKeys.check(runtime!.id, fullName!, defaultBranch!, value.path.trim()),
        result,
      ),
  });

  const current = ready && !clone.isPending ? (check.data ?? null) : null;
  useEffect(() => {
    onCheck(current);
  }, [current, onCheck]);

  useEffect(() => {
    if (check.error) {
      logger.warn("repository checkout check failed", {
        runtimeId: runtime?.id,
        fullName,
        error: check.error.message,
      });
    }
  }, [check.error, runtime?.id, fullName]);

  const commitPath = (path: string) => {
    setDraftPath(null);
    clone.reset();
    if (path !== value.path) onChange({ ...value, path });
  };

  const machineItems = machines
    .filter((m) => m.daemonId)
    .map((m) => ({ value: m.daemonId as string, label: m.title }));

  return (
    <div className="flex flex-col gap-2">
      <Label>{t(($) => $.checkout.label)}</Label>
      {machineItems.length === 0 ? (
        <p className="text-caption text-muted-foreground">{t(($) => $.checkout.no_runtimes)}</p>
      ) : (
        <Select
          items={machineItems}
          value={value.daemonId || null}
          onValueChange={(daemonId) => {
            clone.reset();
            onChange({ ...value, daemonId: (daemonId as string | null) ?? "" });
          }}
          disabled={disabled || !fullName}
        >
          <SelectTrigger aria-label={t(($) => $.checkout.runtime)} className="w-full">
            <SelectValue placeholder={t(($) => $.checkout.runtime_placeholder)}>
              {() =>
                machine
                  ? machine.onlineCount > 0
                    ? t(($) => $.checkout.runtime_online, { name: machine.title })
                    : t(($) => $.checkout.runtime_offline, { name: machine.title })
                  : t(($) => $.checkout.runtime_placeholder)
              }
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {machines.map((m) =>
              m.daemonId ? (
                <SelectItem key={m.id} value={m.daemonId} disabled={m.onlineCount === 0}>
                  {m.onlineCount > 0
                    ? t(($) => $.checkout.runtime_online, { name: m.title })
                    : t(($) => $.checkout.runtime_offline, { name: m.title })}
                </SelectItem>
              ) : null,
            )}
          </SelectContent>
        </Select>
      )}
      <div className="flex items-center gap-2">
        <Input
          id={pathId}
          aria-label={t(($) => $.checkout.path)}
          className="font-mono"
          value={draftPath ?? value.path}
          placeholder={fullName ? undefined : t(($) => $.checkout.pick_repository_first)}
          disabled={disabled || !fullName}
          onChange={(e) => setDraftPath(e.target.value)}
          onBlur={(e) => commitPath(e.currentTarget.value.trim())}
          onKeyDown={(e) => {
            if (e.key !== "Enter") return;
            e.preventDefault();
            commitPath(e.currentTarget.value.trim());
          }}
        />
        <Button
          type="button"
          variant="outline"
          disabled={disabled || !fullName || !runtime}
          onClick={() => setBrowseOpen(true)}
        >
          <FolderOpen />
          {t(($) => $.checkout.choose)}
        </Button>
      </div>
      {fullName && (
        <CheckoutStatus
          fullName={fullName}
          defaultBranch={defaultBranch ?? ""}
          path={value.path.trim()}
          checking={ready && check.isFetching}
          cloning={clone.isPending}
          check={current}
          error={(clone.error ?? check.error)?.message ?? null}
          onClone={() => clone.mutate()}
          onRetry={() => {
            clone.reset();
            void check.refetch();
          }}
        />
      )}
      {runtime && (
        <RuntimeFileBrowser
          open={browseOpen}
          onOpenChange={setBrowseOpen}
          runtime={runtime}
          onAdd={(paths) => {
            const [picked] = paths;
            if (picked) commitPath(picked);
          }}
        />
      )}
    </div>
  );
}

/** The check's outcome under Checkout, with how to fix each failure. */
function CheckoutStatus({
  fullName,
  defaultBranch,
  path,
  checking,
  cloning,
  check,
  error,
  onClone,
  onRetry,
}: {
  fullName: string;
  defaultBranch: string;
  path: string;
  checking: boolean;
  cloning: boolean;
  check: RepositoryCheckoutRequest | null;
  error: string | null;
  onClone: () => void;
  onRetry: () => void;
}) {
  const { t } = useT("repositories");

  if (cloning || checking) {
    return (
      <p className="flex items-center gap-2 text-caption text-muted-foreground" aria-live="polite">
        <Spinner className="size-3.5" />
        {cloning
          ? t(($) => $.checkout.cloning, { name: fullName })
          : t(($) => $.checkout.checking, { path })}
      </p>
    );
  }
  if (error) {
    return (
      <div role="alert" className="flex items-center gap-2 text-caption text-destructive">
        <CircleAlert className="size-3.5 shrink-0" />
        <span className="min-w-0 flex-1">{t(($) => $.checkout.error, { message: error })}</span>
        <Button type="button" variant="outline" size="xs" onClick={onRetry}>
          {t(($) => $.checkout.check_again)}
        </Button>
      </div>
    );
  }
  if (!check) return null;
  if (check.passed) {
    return (
      <div className="flex items-start gap-2 text-caption" aria-live="polite">
        <CircleCheck className="mt-0.5 size-3.5 shrink-0 text-success" />
        <div className="flex flex-col">
          <span>{t(($) => $.checkout.passed, { name: fullName })}</span>
          <span className="text-muted-foreground">
            {t(($) => $.checkout.passed_detail, { branch: defaultBranch })}
          </span>
        </div>
      </div>
    );
  }

  const notFound = check.failures.some((failure) => failure.code === "not_found");
  if (notFound) {
    return (
      <div role="alert" className="flex items-center gap-2 text-caption">
        <CircleAlert className="size-3.5 shrink-0 text-warning" />
        <span className="min-w-0 flex-1">{t(($) => $.checkout.not_found, { path })}</span>
        <Button type="button" variant="outline" size="xs" onClick={onClone}>
          {t(($) => $.checkout.clone)}
        </Button>
      </div>
    );
  }

  return (
    <ul role="alert" className="flex flex-col gap-1.5 text-caption">
      {check.failures.map((failure) => (
        <FailureRow
          key={`${failure.code}:${failure.detail}`}
          failure={failure}
          fullName={fullName}
          defaultBranch={defaultBranch}
          path={path}
          onRetry={onRetry}
        />
      ))}
    </ul>
  );
}

function FailureRow({
  failure,
  fullName,
  defaultBranch,
  path,
  onRetry,
}: {
  failure: RepositoryCheckoutFailure;
  fullName: string;
  defaultBranch: string;
  path: string;
  onRetry: () => void;
}) {
  const { t } = useT("repositories");
  const [sentence, fix] = (() => {
    switch (failure.code) {
      case "not_git":
        return [
          t(($) => $.checkout.failure.not_git, { path }),
          t(($) => $.checkout.failure.not_git_fix, { name: fullName }),
        ];
      case "origin_mismatch":
        return [
          t(($) => $.checkout.failure.origin_mismatch, {
            origin: githubShortLabel(failure.detail),
            name: fullName,
          }),
          t(($) => $.checkout.failure.origin_mismatch_fix, { name: fullName }),
        ];
      case "wrong_branch":
        return [
          t(($) => $.checkout.failure.wrong_branch, {
            branch: failure.detail,
            default_branch: defaultBranch,
          }),
          t(($) => $.checkout.failure.wrong_branch_fix, { default_branch: defaultBranch }),
        ];
      case "clone_failed":
        return [
          t(($) => $.checkout.failure.clone_failed, { detail: failure.detail }),
          t(($) => $.checkout.failure.clone_failed_fix),
        ];
      default:
        return [
          t(($) => $.checkout.failure.unknown, { detail: failure.detail }),
          t(($) => $.checkout.failure.unknown_fix),
        ];
    }
  })();
  return (
    <li className="flex items-start gap-2">
      <CircleAlert className="mt-0.5 size-3.5 shrink-0 text-destructive" />
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="text-destructive">{sentence}</span>
        <span className="text-muted-foreground">{fix}</span>
      </div>
      {failure.code === "wrong_branch" && (
        <Button type="button" variant="outline" size="xs" onClick={onRetry}>
          {t(($) => $.checkout.check_again)}
        </Button>
      )}
    </li>
  );
}
