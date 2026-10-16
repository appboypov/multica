"use client";

import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { clientErrorMessage } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { releaseProgressOptions } from "@multica/core/issues/placement";
import { useIssueStatuses } from "@multica/core/issue-statuses";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  lineStates,
  releaseIssuesOptions,
  releaseTimelineTarget,
  useSetReleaseOwner,
} from "@multica/core/releases";
import { repositoryListOptions } from "@multica/core/repositories";
import type { Release, ReleaseIssue, ReleaseOwnerRequest } from "@multica/core/types";
import { worklaneListOptions } from "@multica/core/worklanes";
import { useActorName } from "@multica/core/workspace/hooks";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../../common/actor-avatar";
import { StatusIcon } from "../../issues/components/status-icon";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";
import { useReleaseLine } from "../use-release-line";
import { ReleaseChip } from "./release-chips";
import { ReleaseComments } from "./release-comments";
import { ReleaseOwnerLabel, ReleaseOwnerPicker } from "./release-owner-picker";
import { ReleaseShippedDate } from "./release-shipped-date";

const NO_ISSUES: ReleaseIssue[] = [];

/**
 * A release's own page: repository, version, state, title, description and
 * owner, its issues in their order on the release, "Open on Timeline" and
 * the discussion. Line edits (edit, move, ship, add issues) stay on the
 * Timeline.
 */
export function ReleaseDetail({ release }: { release: Release }) {
  const wsId = useWorkspaceId();
  const { t } = useT("releases");
  const paths = useWorkspacePaths();
  const ownerLabelId = useId();
  const issuesHeadingId = useId();
  const { data: repositories } = useQuery(repositoryListOptions(wsId));
  const repository = repositories?.find((candidate) => candidate.id === release.repository_id);
  const { line } = useReleaseLine(release.repository_id, repository?.last_tag ?? null);
  const shipped = release.shipped_at !== null;
  const state = lineStates(line).get(release.id) ?? (shipped ? "shipped" : "planned");
  const { data: worklanes } = useQuery(worklaneListOptions(wsId));
  const { data: progress } = useQuery(releaseProgressOptions(wsId));
  const target =
    worklanes && progress ? releaseTimelineTarget({ release, worklanes, progress }) : null;
  const setOwner = useSetReleaseOwner();
  const [ownerFailure, setOwnerFailure] = useState<string | null>(null);

  const changeOwner = async (owner: ReleaseOwnerRequest) => {
    setOwnerFailure(null);
    try {
      await setOwner.mutateAsync({ id: release.id, repositoryId: release.repository_id, owner });
    } catch (err) {
      setOwnerFailure(clientErrorMessage(err) ?? t(($) => $.owner.failed));
    }
  };

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-8 px-6 py-8">
      <header className="flex flex-col gap-3">
        <span className="text-label text-muted-foreground">{repository?.full_name ?? ""}</span>
        <div className="flex flex-wrap items-center gap-2">
          <ReleaseChip version={release.version} state={state} />
          <span className={cn("text-caption", state === "next" ? "text-brand" : "text-muted-foreground")}>
            {t(($) => $.column.state[state])}
          </span>
          <ReleaseShippedDate shippedAt={release.shipped_at} />
        </div>
        <h1 className="text-xl font-semibold">{release.title}</h1>
        {release.description ? (
          <p className="text-body whitespace-pre-wrap text-muted-foreground">{release.description}</p>
        ) : null}
        <div className="flex items-center gap-3 text-label">
          <span id={ownerLabelId} className="text-muted-foreground">
            {t(($) => $.owner.label)}
          </span>
          {shipped ? (
            <ReleaseOwnerLabel
              ownerType={release.owner_type}
              ownerId={release.owner_id}
              ownerName={release.owner_name}
            />
          ) : (
            <ReleaseOwnerPicker
              ownerType={release.owner_type}
              ownerId={release.owner_id}
              ownerName={release.owner_name}
              onChange={(owner) => void changeOwner(owner)}
              labelledBy={ownerLabelId}
              variant="inline"
              pending={setOwner.isPending}
            />
          )}
        </div>
        {ownerFailure !== null ? (
          <p role="alert" className="text-label text-destructive">
            {ownerFailure}
          </p>
        ) : null}
        {target ? (
          <div>
            <AppLink
              href={`${paths.worklaneDetail(target.id)}?release=${encodeURIComponent(release.id)}`}
              className="text-label text-primary hover:underline"
            >
              {t(($) => $.page.open_timeline)}
            </AppLink>
          </div>
        ) : null}
      </header>

      <section aria-labelledby={issuesHeadingId} className="flex flex-col gap-3">
        <h2 id={issuesHeadingId} className="text-body-lg font-medium">
          {t(($) => $.page.issues)}
        </h2>
        <ReleaseIssueList releaseId={release.id} />
      </section>

      <ReleaseComments releaseId={release.id} />
    </div>
  );
}

function ReleaseIssueList({ releaseId }: { releaseId: string }) {
  const wsId = useWorkspaceId();
  const { t } = useT("releases");
  const { data, isLoading } = useQuery(releaseIssuesOptions(wsId, releaseId));
  const issues = data ?? NO_ISSUES;
  if (isLoading) {
    return (
      <div className="flex flex-col gap-2">
        <Skeleton className="h-9 w-full" />
        <Skeleton className="h-9 w-full" />
      </div>
    );
  }
  if (issues.length === 0) {
    return <p className="text-label text-muted-foreground">{t(($) => $.page.no_issues)}</p>;
  }
  return (
    <ol className="flex flex-col divide-y divide-border rounded-lg border border-border">
      {issues.map((issue) => (
        <li key={issue.id}>
          <ReleaseIssueLink issue={issue} />
        </li>
      ))}
    </ol>
  );
}

function ReleaseIssueLink({ issue }: { issue: ReleaseIssue }) {
  const wsId = useWorkspaceId();
  const { t } = useT("releases");
  const paths = useWorkspacePaths();
  const statuses = useIssueStatuses(wsId);
  const { getActorName } = useActorName();
  const assignee =
    issue.assignee_type && issue.assignee_id ? getActorName(issue.assignee_type, issue.assignee_id) : null;
  return (
    <AppLink
      href={paths.issueDetail(issue.id)}
      className="grid grid-cols-[auto_auto_minmax(0,1fr)_minmax(0,10rem)_minmax(0,10rem)] items-center gap-3 px-3 py-2 text-label hover:bg-accent"
    >
      <span className="flex items-center">
        <StatusIcon
          status={issue.status}
          category={statuses.categoryOf(issue.status)}
          color={statuses.colorOf(issue.status)}
          icon={statuses.iconOf(issue.status)}
          className="size-4"
        />
        <span className="sr-only">{statuses.labelOf(issue.status)}</span>
      </span>
      <span className="text-caption text-muted-foreground">{issue.identifier}</span>
      <span className="truncate">{issue.title}</span>
      <span className="flex min-w-0 items-center gap-1.5 text-caption text-muted-foreground">
        {assignee !== null && issue.assignee_type && issue.assignee_id ? (
          <>
            <ActorAvatar actorType={issue.assignee_type} actorId={issue.assignee_id} name={assignee} size="sm" />
            <span className="truncate">{assignee}</span>
          </>
        ) : (
          <span className="truncate">{t(($) => $.page.unassigned)}</span>
        )}
      </span>
      <span className="truncate text-caption text-muted-foreground">
        {issue.worklane_name ?? t(($) => $.page.no_worklane)}
      </span>
    </AppLink>
  );
}
