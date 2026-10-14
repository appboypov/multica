import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { repositoryKeys } from "./queries";
import type { CreateRepositoryRequest, UpdateRepositoryRequest } from "../types";

// The repository mutations await the server: a repository exists only once
// the server read it from GitHub and found its passed check, so nothing is
// shown before it answers. Each invalidates the repositories when it settles;
// the worklanes follow through their own realtime events.

export function useCreateRepository() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateRepositoryRequest) => api.createRepository(data),
    onSettled: () => qc.invalidateQueries({ queryKey: repositoryKeys.all(wsId) }),
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
    onSettled: () => qc.invalidateQueries({ queryKey: repositoryKeys.all(wsId) }),
  });
}
