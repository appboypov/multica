import type { QueryClient, QueryKey } from "@tanstack/react-query";
import type { Todo } from "../types/todo";
import { todoKeys } from "./queries";

async function refreshTodoQuery(qc: QueryClient, queryKey: QueryKey) {
  // A first load already on the wire may predate this event. Invalidation alone
  // reuses that response when no cached data exists, so always cancel first.
  await qc.cancelQueries({ queryKey });
  await qc.invalidateQueries({ queryKey });
}

export async function onTodosInvalidate(qc: QueryClient, wsId: string) {
  await refreshTodoQuery(qc, todoKeys.all(wsId));
}

export async function onTodoSummaryInvalidate(qc: QueryClient) {
  await refreshTodoQuery(qc, todoKeys.summary());
}

export async function onTodoOpened(qc: QueryClient, todo: Todo) {
  // The server decides review/question suppression and overdue ordering. Do not
  // insert a row locally: an open review need not be visible at all.
  await Promise.all([
    onTodosInvalidate(qc, todo.workspace_id),
    onTodoSummaryInvalidate(qc),
  ]);
}

export async function onTodoClosed(qc: QueryClient, id: string) {
  // The close payload intentionally has no workspace id. Remove only this row
  // from existing lists, then refresh the server projections in every cached
  // workspace (a hidden review can become visible when its question closes).
  await qc.cancelQueries({ queryKey: ["todos"] });
  for (const query of qc.getQueryCache().findAll({ queryKey: ["todos"] })) {
    if (query.queryKey[2] === "list") {
      qc.setQueryData<Todo[]>(query.queryKey, (rows) => rows?.filter((row) => row.id !== id));
    }
  }
  await qc.invalidateQueries({ queryKey: ["todos"] });
}
