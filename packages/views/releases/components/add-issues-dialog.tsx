"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { useWorkspaceId } from "@multica/core/hooks";
import { placementRefusalOf, useAddIssuesToRelease } from "@multica/core/issues/placement";
import { formatVersion, releaseListOptions } from "@multica/core/releases";
import type { Release } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../../i18n";
import { ReleaseIssuesField } from "./release-issues-field";

const NO_RELEASES: Release[] = [];

/**
 * Add issue to release: ticks this worklane's issues in no release into an
 * unshipped release, awaiting the server. A refusal keeps the dialog open.
 * `data` holds `releaseId` and `worklaneId`.
 */
export function AddIssuesDialog({
  onClose,
  data,
}: {
  onClose: () => void;
  data: Record<string, unknown> | null;
}) {
  const { t } = useT("releases");
  const { t: tIssues } = useT("issues");
  const releaseId = typeof data?.releaseId === "string" ? data.releaseId : "";
  const worklaneId = typeof data?.worklaneId === "string" ? data.worklaneId : "";
  const wsId = useWorkspaceId();
  const { data: releases = NO_RELEASES } = useQuery(releaseListOptions(wsId));
  const release = releases.find((r) => r.id === releaseId);
  const addIssues = useAddIssuesToRelease();
  const [selected, setSelected] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  const version = release ? formatVersion(release.version) : "";
  const count = selected.length;

  const handleSubmit = async () => {
    if (!release || count === 0) return;
    setError(null);
    try {
      await addIssues.mutateAsync({ releaseId: release.id, issueIds: selected });
      toast.success(t(($) => $.issues.toast_added, { count, version }));
      onClose();
    } catch (err) {
      const refusal = placementRefusalOf(err);
      setError(
        refusal && refusal.code !== "worklane_has_open_issues"
          ? tIssues(($) => $.placement_refusal[refusal.code], {
              version: refusal.version ? formatVersion(refusal.version) : version,
              title: refusal.title ?? release.title,
              issues: refusal.issues.map((entry) => entry.identifier).join(", "),
            })
          : t(($) => $.issues.add_failed),
      );
    }
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {release ? t(($) => $.issues.add_title, { version, title: release.title }) : t(($) => $.issues.add_issue)}
          </DialogTitle>
        </DialogHeader>
        {worklaneId ? (
          <ReleaseIssuesField
            worklaneId={worklaneId}
            selected={selected}
            onChange={(ids) => {
              setSelected(ids);
              setError(null);
            }}
          />
        ) : null}
        {error ? (
          <p role="alert" className="text-caption text-destructive">
            {error}
          </p>
        ) : null}
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            {t(($) => $.issues.cancel)}
          </Button>
          <Button
            onClick={() => void handleSubmit()}
            disabled={!release || count === 0 || addIssues.isPending}
            aria-busy={addIssues.isPending || undefined}
          >
            {addIssues.isPending
              ? t(($) => $.issues.adding)
              : count === 0
                ? t(($) => $.issues.add_submit_none)
                : t(($) => $.issues.add_submit, { count })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
