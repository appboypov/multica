import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import type { CreateWorklaneRequest, WorklaneRequest } from "../types";
import { worklaneKeys } from "./queries";
import { worklaneRequestKeys } from "./requests";

/**
 * Create, retry, rename and remove each await the server: the dialogs show
 * the server's answer, never an optimistic one. A request the server answers
 * with is seeded into its polled query, so polling starts from it.
 */

export function useCreateWorklaneRequest() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateWorklaneRequest) => api.createWorklaneRequest(data),
    onSuccess: (request: WorklaneRequest) =>
      qc.setQueryData(worklaneRequestKeys.detail(wsId, request.id), request),
    onSettled: () => qc.invalidateQueries({ queryKey: worklaneKeys.all(wsId) }),
  });
}

/** Runs a failed request again from its failed step. */
export function useRetryWorklaneRequest() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.retryWorklaneRequest(id),
    onSuccess: (request: WorklaneRequest) =>
      qc.setQueryData(worklaneRequestKeys.detail(wsId, request.id), request),
    onSettled: () => qc.invalidateQueries({ queryKey: worklaneKeys.all(wsId) }),
  });
}

export function useRenameWorklane() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) => api.renameWorklane(id, { name }),
    onSettled: () => qc.invalidateQueries({ queryKey: worklaneKeys.all(wsId) }),
  });
}

/** Starts the remove request on the worklane's machine; the worklane goes once it reports removed. */
export function useRemoveWorklane() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.removeWorklane(id),
    onSuccess: (request: WorklaneRequest) =>
      qc.setQueryData(worklaneRequestKeys.detail(wsId, request.id), request),
    onSettled: () => qc.invalidateQueries({ queryKey: worklaneKeys.all(wsId) }),
  });
}
