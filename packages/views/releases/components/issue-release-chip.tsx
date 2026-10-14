"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { lineStates, releaseListOptions, selectLine, toLine } from "@multica/core/releases";
import { ReleaseChip } from "./release-chips";

/**
 * The release of an issue as its version in its state's colour: next,
 * planned or shipped on its repository's line. Nothing while the workspace's
 * releases load or when the release is gone.
 */
export function IssueReleaseChip({ releaseId }: { releaseId: string }) {
  const wsId = useWorkspaceId();
  const { data: releases } = useQuery(releaseListOptions(wsId));
  const placed = useMemo(() => {
    const release = releases?.find((candidate) => candidate.id === releaseId);
    if (!releases || !release) return null;
    const states = lineStates(toLine(selectLine(releases, release.repository_id), null));
    return { release, state: states.get(release.id) ?? "planned" };
  }, [releases, releaseId]);

  if (!placed) return null;
  return <ReleaseChip version={placed.release.version} state={placed.state} />;
}
