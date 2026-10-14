"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { openedFrom } from "@multica/core/issues/placement";
import type { IssueScope } from "@multica/core/issues/surface/scope";
import { releaseListOptions } from "@multica/core/releases";
import { repositoryListOptions } from "@multica/core/repositories";
import { worklaneListOptions, type IssueTypePick } from "@multica/core/worklanes";
import { IssueSurface } from "../../issues/surface/issue-surface";
import type { IssueCreateDefaults } from "../../issues/surface/types";

/**
 * A worklane's Board: its issues by status, Cancelled hidden by default. The
 * surface key is the worklane's own, so the List shares its card properties,
 * and the picked types narrow the query without changing the key. A column's
 * + opens New issue in the worklane and its repository's next release.
 */
export function WorklaneBoard({
  worklaneId,
  issueTypes,
}: {
  worklaneId: string;
  issueTypes: readonly IssueTypePick[];
}) {
  const wsId = useWorkspaceId();
  const { data: worklanes = [] } = useQuery(worklaneListOptions(wsId));
  const { data: repositories = [] } = useQuery(repositoryListOptions(wsId));
  const { data: releases = [] } = useQuery(releaseListOptions(wsId));

  const scope = useMemo<IssueScope>(
    () => ({ type: "worklane", worklaneId, issueTypes: [...issueTypes] }),
    [worklaneId, issueTypes],
  );
  const createDefaults = useMemo<IssueCreateDefaults>(() => {
    const placement = openedFrom({ worklaneId }, { worklanes, repositories, releases });
    return { worklane_id: placement.worklaneId, release_id: placement.releaseId };
  }, [worklaneId, worklanes, repositories, releases]);

  return (
    <IssueSurface
      scope={scope}
      modes={["board"]}
      surfaceKey={`worklane:${worklaneId}`}
      createDefaults={createDefaults}
      announceMoves
      openMode="peek"
    />
  );
}
