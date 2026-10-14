import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { api, ApiError } from "../../api";
import { IssuePlacementRefusalSchema } from "../../api/schemas";
import { useWorkspaceId } from "../../hooks";
import type { Issue, IssuePlacementRefusal, IssueType, SetIssuePlacementRequest } from "../../types";
import { issueKeys } from "../queries";
import { onIssueUpdated } from "../ws-updaters";
import { invalidatePlacementReads } from "./queries";

/** The refusal of a type or placement write (design.md decision 2), or null for any other failure. */
export function placementRefusalOf(error: unknown): IssuePlacementRefusal | null {
  if (!(error instanceof ApiError) || (error.status !== 400 && error.status !== 409)) return null;
  const parsed = IssuePlacementRefusalSchema.safeParse(error.body);
  return parsed.success ? parsed.data : null;
}

function applyAnsweredIssue(qc: QueryClient, wsId: string, issue: Issue) {
  qc.setQueryData<Issue>(issueKeys.detail(wsId, issue.id), (old) =>
    old && old.revision !== undefined && issue.revision !== undefined && old.revision > issue.revision
      ? old
      : { ...old, ...issue },
  );
  onIssueUpdated(qc, wsId, issue);
  qc.invalidateQueries({ queryKey: issueKeys.timeline(issue.id) });
}

/** Sets an issue's type once, awaiting the server. A set type answers `type_fixed`. */
export function useSetIssueType() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, type }: { id: string; type: IssueType }) => api.setIssueType(id, type),
    onSuccess: (issue) => applyAnsweredIssue(qc, wsId, issue),
    onSettled: (_issue, _err, vars) => {
      qc.invalidateQueries({ queryKey: issueKeys.detail(wsId, vars.id) });
      invalidatePlacementReads(qc, wsId);
    },
  });
}

/** Places an issue in a worklane and a release, awaiting the server. */
export function useSetIssuePlacement() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, ...placement }: { id: string } & SetIssuePlacementRequest) =>
      api.setIssuePlacement(id, placement),
    onSuccess: (issue) => applyAnsweredIssue(qc, wsId, issue),
    onSettled: (_issue, _err, vars) => {
      qc.invalidateQueries({ queryKey: issueKeys.detail(wsId, vars.id) });
      invalidatePlacementReads(qc, wsId);
    },
  });
}

/** Adds issues of a worklane to an unshipped release, awaiting the server. */
export function useAddIssuesToRelease() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ releaseId, issueIds }: { releaseId: string; issueIds: string[] }) =>
      api.addIssuesToRelease(releaseId, issueIds),
    onSuccess: (issues) => {
      for (const issue of issues) applyAnsweredIssue(qc, wsId, issue);
    },
    onSettled: () => invalidatePlacementReads(qc, wsId),
  });
}
