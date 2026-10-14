"use client";

import { useEffect, useMemo } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { releaseListOptions, selectLine, toLine, type Line } from "@multica/core/releases";
import { repositoryCheckoutOptions, repositoryKeys } from "@multica/core/repositories";
import { runtimeListOptions } from "@multica/core/runtimes";
import type { Release, Repository } from "@multica/core/types";

const NO_RELEASES: Release[] = [];

/**
 * A repository's releases in line order, and the line the rules run on. The
 * line's last tag is "" while the repository's last tag is not known yet;
 * callers check `repository.last_tag === null` for that state.
 */
export function useReleaseLine(repositoryId: string, lastTag: string | null): {
  releases: Release[];
  line: Line;
  isLoading: boolean;
} {
  const wsId = useWorkspaceId();
  const { data = NO_RELEASES, isLoading } = useQuery(releaseListOptions(wsId));
  const releases = selectLine(data, repositoryId);
  const line = useMemo(() => toLine(releases, lastTag), [releases, lastTag]);
  return { releases, line, isLoading };
}

export type LastTagState = "known" | "finding" | "unknown";

/**
 * While a repository's last tag is not known, runs the checkout check on its
 * checkout machine: a passing check makes the server store the tag and
 * publish `repository:updated` (design.md decision 5). "unknown" means the
 * check could not run or did not pass, so the server's `last_tag_unknown`
 * refusal stays the gate.
 */
export function useLastTag(repository: Repository | undefined): LastTagState {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const missing = !!repository && repository.last_tag === null && repository.version_policy !== "none";
  const { data: runtimes, isLoading: runtimesLoading } = useQuery({
    ...runtimeListOptions(wsId, "me"),
    enabled: missing,
  });
  const runtime = runtimes?.find(
    (r) => r.daemon_id === repository?.checkout_daemon_id && r.status === "online",
  );
  const check = useQuery({
    ...repositoryCheckoutOptions(
      runtime?.id ?? "",
      repository?.full_name ?? "",
      repository?.default_branch ?? "",
      repository?.checkout_path ?? "",
    ),
    enabled: missing && !!runtime,
  });
  const passed = check.data?.passed === true;
  useEffect(() => {
    if (passed) void qc.invalidateQueries({ queryKey: repositoryKeys.list(wsId) });
  }, [passed, qc, wsId]);

  if (!missing) return "known";
  if (runtimesLoading || check.isLoading || check.isFetching || passed) return "finding";
  return "unknown";
}
