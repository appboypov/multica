import { useMutation, useQueryClient, type UseMutationResult } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { worklaneIssueKeys } from "../issues/placement/queries";
import type { Issue } from "../types";
import { worklaneKeys } from "./queries";

export type LaneMove = { issueId: string } & ({ before: string; after?: never } | { after: string; before?: never });

export interface LaneMoveContext { previous: Issue[] | undefined }
export type LaneMoveMutation = UseMutationResult<Issue[], Error, LaneMove, LaneMoveContext>;
/** Release order stays fixed; only the order within one release changes. */
export function moveLaneIssue(issues: readonly Issue[], move: LaneMove): Issue[] {
  const moving = issues.find((issue) => issue.id === move.issueId);
  const targetId = move.before ?? move.after;
  const target = issues.find((issue) => issue.id === targetId);
  if (!moving || !target || moving.id === target.id || moving.release_id !== target.release_id) return [...issues];
  const result = issues.filter((issue) => issue.id !== moving.id);
  const index = result.findIndex((issue) => issue.id === target.id) + (move.after ? 1 : 0);
  result.splice(index, 0, moving);
  return result;
}

export function useMoveWorklaneIssue(worklaneId: string): LaneMoveMutation {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  const queryKey = worklaneIssueKeys.list(wsId, worklaneId);
  return useMutation({
    mutationFn: (move: LaneMove) => api.moveWorklaneIssue(worklaneId, move.issueId, move),
    onMutate: async (move) => {
      await qc.cancelQueries({ queryKey });
      const previous = qc.getQueryData<Issue[]>(queryKey);
      if (previous) qc.setQueryData(queryKey, moveLaneIssue(previous, move));
      return { previous };
    },
    onError: (_error, _move, context) => {
      if (context?.previous) qc.setQueryData(queryKey, context.previous);
    },
    onSuccess: (issues) => qc.setQueryData(queryKey, issues),
    onSettled: () => {
      qc.invalidateQueries({ queryKey });
      qc.invalidateQueries({ queryKey: worklaneKeys.all(wsId) });
    },
  });
}
