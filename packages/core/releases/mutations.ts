import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { api, ApiError, parseWithFallback } from "../api";
import { ReleaseRefusalSchema, UNKNOWN_RELEASE_REFUSAL } from "../api/schemas";
import { useWorkspaceId } from "../hooks";
import type {
  CreateReleaseCommentRequest,
  EditReleaseRequest,
  ListReleasesResponse,
  PlanReleaseRequest,
  Release,
  ReleaseComment,
  ReleaseLineResponse,
  ReleaseOwnerRequest,
  ReleaseRefusal,
  ReleaseMilestoneRequest,
} from "../types";
import { worklaneKeys } from "../worklanes/queries";
import { releaseKeys } from "./queries";
import { invalidatePlacementReads, releaseProgressKeys } from "../issues/placement/queries";

/**
 * Every write answers with the repository's whole line: it replaces that
 * repository's releases in the workspace list and in open release pages
 * without a second read. Refresh progress metadata, the repository's
 * worklane activity and the releases' owner activity.
 */
function settleLine(
  qc: QueryClient,
  wsId: string,
  repositoryId: string,
  answer: ReleaseLineResponse,
) {
  qc.setQueryData<ListReleasesResponse>(releaseKeys.list(wsId), (old) =>
    old
      ? {
          ...old,
          releases: [
            ...old.releases.filter((release) => release.repository_id !== repositoryId),
            ...answer.releases,
          ],
        }
      : old,
  );
  for (const release of answer.releases) {
    qc.setQueryData<Release>(releaseKeys.detail(wsId, release.id), (old) => (old ? release : old));
  }
  qc.invalidateQueries({ queryKey: worklaneKeys.activityAll(wsId) });
  qc.invalidateQueries({ queryKey: releaseKeys.activityAll(wsId) });
  qc.invalidateQueries({ queryKey: releaseProgressKeys.list(wsId) });
}

/** The structured refusal of a failed release write, or null for any other error. */
export function releaseRefusalOf(error: unknown): ReleaseRefusal | null {
  if (!(error instanceof ApiError) || (error.status !== 400 && error.status !== 409)) return null;
  return parseWithFallback(error.body, ReleaseRefusalSchema, UNKNOWN_RELEASE_REFUSAL, {
    endpoint: "release refusal",
  });
}

export function usePlanRelease() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ repositoryId, data }: { repositoryId: string; data: PlanReleaseRequest }) =>
      api.planRelease(repositoryId, data),
    onSuccess: (answer, { repositoryId }) => settleLine(qc, wsId, repositoryId, answer),
    // A plan with `issue_ids` places those issues in the new release.
    onSettled: () => invalidatePlacementReads(qc, wsId),
  });
}

export function useEditRelease() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; repositoryId: string; data: EditReleaseRequest }) =>
      api.editRelease(id, data),
    onSuccess: (answer, { repositoryId }) => settleLine(qc, wsId, repositoryId, answer),
  });
}

/** Sets, changes or removes (both null) the owner of an unshipped release, once the server agrees. */
export function useSetReleaseOwner() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, owner }: { id: string; repositoryId: string; owner: ReleaseOwnerRequest }) =>
      api.setReleaseOwner(id, owner),
    onSuccess: (answer, { repositoryId }) => settleLine(qc, wsId, repositoryId, answer),
  });
}

/**
 * Posts a comment or reply on a release's page. The caller keeps the text
 * until this succeeds, so a failed send can be retried without retyping.
 */
export function useCreateReleaseComment(releaseId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateReleaseCommentRequest) => api.createReleaseComment(releaseId, data),
    onSuccess: (comment) => {
      if (!comment.id) return;
      qc.setQueryData<ReleaseComment[]>(releaseKeys.comments(wsId, releaseId), (old) =>
        old && !old.some((existing) => existing.id === comment.id) ? [...old, comment] : old,
      );
    },
    onSettled: () => qc.invalidateQueries({ queryKey: releaseKeys.comments(wsId, releaseId) }),
  });
}

export function useMoveRelease() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({
      id,
      direction,
    }: {
      id: string;
      repositoryId: string;
      direction: "earlier" | "later";
    }) => api.moveRelease(id, { direction }),
    onSuccess: (answer, { repositoryId }) => settleLine(qc, wsId, repositoryId, answer),
  });
}

export function useRemoveRelease() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id }: { id: string; repositoryId: string }) => api.removeRelease(id),
    onSuccess: (answer, { repositoryId }) => settleLine(qc, wsId, repositoryId, answer),
    // The removed release's issues go to No release.
    onSettled: () => invalidatePlacementReads(qc, wsId),
  });
}

export function useAddReleaseMilestone() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ releaseId, data }: { releaseId: string; repositoryId: string; data: ReleaseMilestoneRequest }) =>
      api.addReleaseMilestone(releaseId, data),
    onSuccess: (answer, { repositoryId }) => settleLine(qc, wsId, repositoryId, answer),
  });
}

export function useEditReleaseMilestone() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; repositoryId: string; data: Partial<ReleaseMilestoneRequest> }) =>
      api.editReleaseMilestone(id, data),
    onSuccess: (answer, { repositoryId }) => settleLine(qc, wsId, repositoryId, answer),
  });
}

export function useRemoveReleaseMilestone() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id }: { id: string; repositoryId: string }) => api.removeReleaseMilestone(id),
    onSuccess: (answer, { repositoryId }) => settleLine(qc, wsId, repositoryId, answer),
  });
}
