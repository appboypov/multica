import { useQueries, useQuery } from "@tanstack/react-query";
import type { Squad } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  squadDetailOptions,
  squadListOptions,
} from "@multica/core/workspace/queries";

/**
 * Resolves squads by id. The workspace squad list omits archived squads, so
 * an id that is not in the loaded list is fetched on its own (archived squads
 * included); listed squads cost no extra request.
 *
 * `squads` is the workspace list itself, for pickers that offer only live
 * squads. `isLoading` is true while the list or any per-id fetch is pending.
 */
export function useSquadLookup(squadIds: readonly string[]) {
  const wsId = useWorkspaceId();
  const { data: listed, isLoading: listLoading } = useQuery(
    squadListOptions(wsId),
  );
  const missingIds = listed
    ? [...new Set(squadIds)].filter((id) => !listed.some((s) => s.id === id))
    : [];
  const details = useQueries({
    queries: missingIds.map((id) => squadDetailOptions(wsId, id)),
  });

  const byId = new Map<string, Squad>();
  for (const squad of listed ?? []) byId.set(squad.id, squad);
  details.forEach((detail, i) => {
    if (detail.data) byId.set(missingIds[i]!, detail.data);
  });

  return {
    squads: listed ?? [],
    getSquad: (id: string): Squad | undefined => byId.get(id),
    isLoading: listLoading || details.some((d) => d.isLoading),
  };
}
