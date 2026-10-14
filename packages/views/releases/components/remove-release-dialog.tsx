"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { clientErrorMessage } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  formatVersion,
  previewRemove,
  releaseRefusalOf,
  useRemoveRelease,
} from "@multica/core/releases";
import { repositoryListOptions } from "@multica/core/repositories";
import type { Repository } from "@multica/core/types";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { useT } from "../../i18n";
import { useReleaseLine } from "../use-release-line";
import { refusalMessage } from "./release-messages";

const NO_REPOSITORIES: Repository[] = [];

/**
 * Remove release's confirmation (the `remove-release` spec delta): the
 * release leaves its repository's line and the Timeline of every worklane of
 * the repository, and the later computed versions renumber.
 */
export function RemoveReleaseDialog({
  onClose,
  data,
}: {
  onClose: () => void;
  data: Record<string, unknown> | null;
}) {
  const { t } = useT("releases");
  const repositoryId = typeof data?.repositoryId === "string" ? data.repositoryId : "";
  const releaseId = typeof data?.releaseId === "string" ? data.releaseId : "";
  const wsId = useWorkspaceId();
  const { data: repositories = NO_REPOSITORIES } = useQuery(repositoryListOptions(wsId));
  const repository = repositories.find((r) => r.id === repositoryId);
  const { releases, line } = useReleaseLine(repositoryId, repository?.last_tag ?? null);
  const release = releases.find((r) => r.id === releaseId);
  const removeRelease = useRemoveRelease();
  const [refusal, setRefusal] = useState<string | null>(null);

  const changes = release ? previewRemove(line, release.id).changes : [];
  const version = release ? formatVersion(release.version) : "";
  const title = release?.title ?? "";

  const handleRemove = async () => {
    if (!release || !repository) return;
    setRefusal(null);
    try {
      await removeRelease.mutateAsync({ id: release.id, repositoryId: repository.id });
      onClose();
      toast.success(t(($) => $.remove.toast_removed, { version, title }));
    } catch (err) {
      const answer = releaseRefusalOf(err);
      setRefusal(
        answer
          ? refusalMessage(t, answer, repository.full_name)
          : (clientErrorMessage(err) ?? t(($) => $.remove.failed)),
      );
    }
  };

  return (
    <AlertDialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {release ? t(($) => $.remove.title, { version, title }) : t(($) => $.menu.remove)}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {release && repository
              ? t(($) => $.remove.description, { repository: repository.full_name })
              : t(($) => $.dialog.not_found)}
          </AlertDialogDescription>
        </AlertDialogHeader>
        {changes.length > 0 ? (
          <p className="text-caption text-muted-foreground">
            {t(($) => $.remove.renumber, {
              changes: changes
                .map((change) =>
                  t(($) => $.dialog.renumber_change, {
                    from: formatVersion(change.from),
                    to: formatVersion(change.to),
                  }),
                )
                .join(", "),
            })}
          </p>
        ) : null}
        {refusal ? (
          <p role="alert" className="text-caption text-destructive">
            {refusal}
          </p>
        ) : null}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={removeRelease.isPending}>
            {t(($) => $.remove.cancel)}
          </AlertDialogCancel>
          <AlertDialogAction
            onClick={() => void handleRemove()}
            disabled={!release || removeRelease.isPending}
            className="bg-destructive text-white hover:bg-destructive/90"
          >
            {removeRelease.isPending ? t(($) => $.remove.removing) : t(($) => $.remove.confirm)}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
