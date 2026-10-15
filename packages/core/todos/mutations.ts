import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { createLogger } from "../logger";
import type { Todo, TodoWorkspaceCount } from "../types/todo";
import { todoKeys } from "./queries";
import { onTodosInvalidate, onTodoSummaryInvalidate } from "./ws-updaters";

const logger = createLogger("todos");

export function useMarkTodoDone() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationKey: todoKeys.done(wsId),
    mutationFn: (id: string) => api.markTodoDone(id),
    onMutate: async (id) => {
      await Promise.all([
        qc.cancelQueries({ queryKey: todoKeys.all(wsId) }),
        qc.cancelQueries({ queryKey: todoKeys.summary() }),
      ]);
      const rows = qc.getQueryData<Todo[]>(todoKeys.list(wsId));
      const removed = rows?.find((row) => row.id === id);
      if (removed) qc.setQueryData(todoKeys.list(wsId), rows?.filter((row) => row.id !== id));
      const previousSummary = qc.getQueryData<TodoWorkspaceCount[]>(todoKeys.summary());
      // A delta, never a count rebuilt from a potentially stale list. A repeated
      // click has no cached row to remove and must not lower the badge twice.
      const optimisticSummary = removed && previousSummary
        ? previousSummary.map((entry) => entry.workspace_id === wsId
          ? { ...entry, count: Math.max(0, entry.count - 1) } : entry)
        : previousSummary;
      const appliedSummary = optimisticSummary
        ? qc.setQueryData(todoKeys.summary(), optimisticSummary) : undefined;
      return { removed, previousSummary, optimisticSummary: appliedSummary };
    },
    onError: (error, id, context) => {
      logger.warn("Mark done failed", { todoId: id, error });
      if (context?.removed) {
        const removed = context.removed;
        qc.setQueryData<Todo[]>(todoKeys.list(wsId), (rows) => {
          if (rows?.some((row) => row.id === id)) return rows;
          return [...(rows ?? []), removed].sort((a, b) =>
            Number(b.overdue) - Number(a.overdue) || b.created_at.localeCompare(a.created_at));
        });
      }
      // Do not overwrite a concurrent mutation or a realtime/server refresh.
      // Any newer count is reconciled by the authoritative read on settle.
      if (context?.previousSummary && qc.getQueryData(todoKeys.summary()) === context.optimisticSummary) {
        qc.setQueryData(todoKeys.summary(), context.previousSummary);
      }
    },
    onSettled: () => {
      // The last pending Mark done refreshes; an earlier response must not
      // resurrect rows optimistically removed by another pending mutation.
      if (qc.isMutating({ mutationKey: todoKeys.done(wsId) }) === 1) {
        void onTodosInvalidate(qc, wsId);
        void onTodoSummaryInvalidate(qc);
      }
    },
  });
}
