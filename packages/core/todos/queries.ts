import { queryOptions, useQuery } from "@tanstack/react-query";
import { api } from "../api";
import type { TodoWorkspaceCount } from "../types/todo";

export const todoKeys = {
  all: (wsId: string) => ["todos", wsId] as const,
  list: (wsId: string) => [...todoKeys.all(wsId), "list"] as const,
  summary: () => ["todos", "summary"] as const,
  done: (wsId: string) => [...todoKeys.all(wsId), "done"] as const,
};

export function todoListOptions(wsId: string) {
  return queryOptions({
    queryKey: todoKeys.list(wsId),
    queryFn: ({ signal }) => api.listTodos(signal),
    enabled: !!wsId,
  });
}

export function todoSummaryOptions() {
  return queryOptions({
    queryKey: todoKeys.summary(),
    queryFn: ({ signal }) => api.getTodoSummary(signal),
  });
}

export function todoCountForWorkspace(summary: TodoWorkspaceCount[], wsId: string | null | undefined): number {
  return summary.find((entry) => entry.workspace_id === wsId)?.count ?? 0;
}

export function hasOtherWorkspaceTodos(summary: TodoWorkspaceCount[], wsId: string | null | undefined): boolean {
  return summary.some((entry) => entry.workspace_id !== wsId && entry.count > 0);
}

export function todoWorkspaceIds(summary: TodoWorkspaceCount[]): Set<string> {
  return new Set(summary.filter((entry) => entry.count > 0).map((entry) => entry.workspace_id));
}

export function useTodoCount(wsId: string | null | undefined): number {
  const { data } = useQuery({
    ...todoSummaryOptions(),
    enabled: !!wsId,
    select: (summary) => todoCountForWorkspace(summary, wsId),
  });
  return data ?? 0;
}
