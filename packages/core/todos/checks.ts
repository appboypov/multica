import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import type { CheckAnswer } from "../types/todo";
import { todoKeys } from "./queries";

export function useProjectCheckTrust(projectId: string) {
  const wsId = useWorkspaceId();
  return useQuery({
    queryKey: [...todoKeys.all(wsId), "trust", projectId],
    queryFn: ({ signal }) => api.getProjectCheckTrust(projectId, signal),
    enabled: !!wsId && !!projectId,
  });
}

export function useAnswerCheckTodo() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, answer, note }: { id: string; answer: CheckAnswer; note: string }) => api.answerCheckTodo(id, answer, note),
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: todoKeys.all(wsId) }),
        qc.invalidateQueries({ queryKey: todoKeys.summary() }),
      ]);
    },
  });
}

export function useSetCheckSampling() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ projectId, kind, sampling }: { projectId: string; kind: string; sampling: boolean }) => api.setCheckSampling(projectId, kind, sampling),
    onSuccess: async () => { await qc.invalidateQueries({ queryKey: todoKeys.all(wsId) }); },
  });
}

export function useCheckHistory(projectId: string, kind: string, enabled = true) {
  const wsId = useWorkspaceId();
  return useQuery({
    queryKey: [...todoKeys.all(wsId), "history", projectId, kind],
    queryFn: ({ signal }) => api.getCheckHistory(projectId, kind, signal),
    enabled: !!wsId && !!projectId && enabled,
  });
}
