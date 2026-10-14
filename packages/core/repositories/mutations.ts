import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { worklaneKeys } from "../worklanes/queries";
import { repositoryKeys } from "./queries";
import type { CreateRepositoryRequest, UpdateRepositoryRequest } from "../types";

// The repository mutations await the server: a repository exists only once
// the server read it from GitHub and found its passed check, so nothing is
// shown before it answers. Each invalidates the repositories when it settles.

/** Creating or removing a repository also makes or removes its worklane `main`. */
function invalidateRepositoriesAndWorklanes(qc: QueryClient, wsId: string) {
  void qc.invalidateQueries({ queryKey: repositoryKeys.all(wsId) });
  void qc.invalidateQueries({ queryKey: worklaneKeys.all(wsId) });
}

export function useCreateRepository() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateRepositoryRequest) => api.createRepository(data),
    onSettled: () => invalidateRepositoriesAndWorklanes(qc, wsId),
  });
}

export function useUpdateRepository() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & UpdateRepositoryRequest) =>
      api.updateRepository(id, data),
    onSettled: () => qc.invalidateQueries({ queryKey: repositoryKeys.all(wsId) }),
  });
}

export function useDeleteRepository() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.deleteRepository(id),
    onSettled: () => invalidateRepositoriesAndWorklanes(qc, wsId),
  });
}
