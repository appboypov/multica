"use client";

import { useQuery } from "@tanstack/react-query";
import { GitBranch } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type { Issue, UpdateIssueRequest } from "@multica/core/types";
import { worklaneListOptions } from "@multica/core/worklanes";
import { PillButton } from "../../common/pill-button";
import { AppLink } from "../../navigation";
import { IssueReleaseChip } from "../../releases/components/issue-release-chip";
import { useT } from "../../i18n";
import { AssigneePicker, StatusPicker } from "./pickers";

/**
 * Where an issue stands, at the top of its preview under the title: a status
 * menu, its worklane as a link to the worklane's Timeline or "No worklane",
 * its release chip, and its assignee.
 */
export function IssuePeekSummary({
  issue,
  onUpdate,
  onMarkDuplicate,
  isDuplicate,
}: {
  issue: Issue;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
  onMarkDuplicate?: () => void;
  isDuplicate?: boolean;
}) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { data: worklanes } = useQuery(worklaneListOptions(wsId));
  const worklane = issue.worklane_id
    ? worklanes?.find((candidate) => candidate.id === issue.worklane_id)
    : undefined;

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <StatusPicker
        status={issue.status}
        onUpdate={onUpdate}
        triggerRender={<PillButton />}
        align="start"
        onMarkDuplicate={onMarkDuplicate}
        isDuplicate={isDuplicate}
      />
      {worklane ? (
        <AppLink
          href={paths.worklaneDetail(worklane.id)}
          className="inline-flex min-w-0 max-w-56 items-center gap-1.5 rounded-full border px-2.5 py-1 text-caption hover:bg-accent/60"
        >
          <GitBranch aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" />
          <span className="truncate">{worklane.name}</span>
        </AppLink>
      ) : (
        !issue.worklane_id && (
          <span className="inline-flex items-center gap-1.5 rounded-full border border-dashed px-2.5 py-1 text-caption text-muted-foreground">
            <GitBranch aria-hidden="true" className="size-3.5 shrink-0" />
            {t(($) => $.peek.no_worklane)}
          </span>
        )
      )}
      {issue.release_id && <IssueReleaseChip releaseId={issue.release_id} />}
      <AssigneePicker
        assigneeType={issue.assignee_type}
        assigneeId={issue.assignee_id}
        onUpdate={onUpdate}
        triggerRender={<PillButton />}
        align="start"
      />
    </div>
  );
}
