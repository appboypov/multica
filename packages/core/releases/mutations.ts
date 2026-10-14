import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { api, ApiError, parseWithFallback } from "../api";
import { ReleaseRefusalSchema, UNKNOWN_RELEASE_REFUSAL } from "../api/schemas";
import { useWorkspaceId } from "../hooks";
import type {
  EditReleaseRequest,
  ListReleasesResponse,
  PlanReleaseRequest,
  ReleaseLineResponse,
  ReleaseRefusal,
} from "../types";
import { releaseKeys } from "./queries";

/**
 * Every write answers with the repository's whole line: it replaces that
 * repository's releases in the workspace list without a second read.
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
  });
}
