"use client";

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { FolderGit2 } from "lucide-react";
import type { Repository, RepositoryCheckoutRequest, VersionPolicy } from "@multica/core/types";
import { clientErrorMessage } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { releaseListOptions, selectLine } from "@multica/core/releases";
import {
  repositoryListOptions,
  repositoryRefusalOf,
  useUpdateRepository,
} from "@multica/core/repositories";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Label } from "@multica/ui/components/ui/label";
import { useT } from "../../i18n";
import { CheckoutField, type CheckoutValue } from "./checkout-field";
import { ReleasesField } from "./releases-field";

/**
 * Edit repository: the repository stays fixed; its Checkout and Releases are
 * filled in. Save sends a check id only when the checkout changed, and then
 * only once the new checkout passes its check.
 */
export function EditRepositoryDialog({
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
  return <EditRepositoryForm repository={repository} onClose={onClose} />;
}

function EditRepositoryForm({
  repository,
  onClose,
}: {
  repository: Repository;
  onClose: () => void;
}) {
  const { t } = useT("repositories");
  const updateRepository = useUpdateRepository();
  const [checkout, setCheckout] = useState<CheckoutValue>({
    daemonId: repository.checkout_daemon_id,
    path: repository.checkout_path,
  });
  const [check, setCheck] = useState<RepositoryCheckoutRequest | null>(null);
  const [policy, setPolicy] = useState<VersionPolicy | null>(repository.version_policy);
  const [serverError, setServerError] = useState<string | null>(null);
  const wsId = useWorkspaceId();
  const { data: releases } = useQuery(releaseListOptions(wsId));
  const lineUnshipped = selectLine(releases ?? [], repository.id).filter((r) => r.shipped_at === null);
  // The server's has_unshipped_releases answer wins over the cached line.
  const [refusedReleases, setRefusedReleases] = useState<{ version: string; title: string }[] | null>(null);
  const unshipped = refusedReleases ?? lineUnshipped;

  const checkoutChanged =
    checkout.daemonId !== repository.checkout_daemon_id ||
    checkout.path.trim() !== repository.checkout_path;
  const policyChanged = policy !== null && policy !== repository.version_policy;
  const stopsReleasingTooEarly = policy === "none" && unshipped.length > 0;
  const canSave =
    (checkoutChanged ? !!check?.passed : policyChanged) &&
    policy !== null &&
    !stopsReleasingTooEarly &&
    !updateRepository.isPending;

  const handleSave = async () => {
    if (!canSave || !policy) return;
    setServerError(null);
    setRefusedReleases(null);
    try {
      await updateRepository.mutateAsync({
        id: repository.id,
        version_policy: policy,
        ...(checkoutChanged && check ? { checkout_check_id: check.id } : {}),
      });
      onClose();
      toast.success(t(($) => $.edit.toast_saved, { name: repository.full_name }));
    } catch (err) {
      const refusal = repositoryRefusalOf(err);
      if (refusal?.code === "has_unshipped_releases") {
        setRefusedReleases(refusal.releases);
        return;
      }
      setServerError(clientErrorMessage(err) ?? t(($) => $.edit.failed));
    }
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t(($) => $.edit.title)}</DialogTitle>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label>{t(($) => $.edit.repository)}</Label>
            <div className="flex items-center gap-2 rounded-lg border bg-muted/40 px-3 py-2">
              <FolderGit2 aria-hidden className="size-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1 truncate font-mono text-body">
                {repository.full_name}
              </span>
              <span className="shrink-0 text-caption text-muted-foreground">
                {t(($) => $.picker.meta, {
                  visibility: repository.private
                    ? t(($) => $.picker.private)
                    : t(($) => $.picker.public),
                  branch: repository.default_branch,
                })}
              </span>
            </div>
          </div>

          <CheckoutField
            fullName={repository.full_name}
            defaultBranch={repository.default_branch}
            value={checkout}
            onChange={setCheckout}
            onCheck={setCheck}
          />

          <ReleasesField
            versionFiles={check ? check.version_files : null}
            lastTag={check ? check.last_tag : repository.last_tag}
            value={policy}
            onChange={(next) => {
              setPolicy(next);
              setRefusedReleases(null);
            }}
            keep={repository.version_policy}
            unshipped={unshipped}
          />

          {serverError ? (
            <p role="alert" className="text-caption text-destructive">
              {serverError}
            </p>
          ) : null}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            {t(($) => $.edit.cancel)}
          </Button>
          <Button
            type="button"
            disabled={!canSave}
            aria-busy={updateRepository.isPending || undefined}
            onClick={() => void handleSave()}
          >
            {updateRepository.isPending ? t(($) => $.edit.submitting) : t(($) => $.edit.submit)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
