import type {
  CreateIssueRequest,
  IssueTableFilters,
  IssueTableScope,
} from "../../types";
import type { MyIssuesFilter } from "../queries";
import {
  assigneeTypesForActorKind,
  issueScopeKey,
  UnsupportedIssueScopeError,
  type IssueScope,
} from "./scope";

/**
 * The scope's non-Table residue. Row membership for every list-shaped mode
 * (table, list, board, swimlane) is compiled into an IssueTableQuerySpec by
 * the surface controller and answered by the server-owned Table channel —
 * this plan only carries what that channel does not cover:
 *
 * - `scopeKey`: the surface's cache/persistence identity.
 * - `queryFilter`: the scope as legacy list-API params, consumed solely by
 *   the Gantt projection (whose scheduled-only window is not expressible in
 *   the Table spec).
 * - `createDefaults`: what a new issue created on this surface inherits.
 */
export interface IssueSurfaceQueryPlan {
  scopeKey: string;
  queryFilter: MyIssuesFilter;
  createDefaults: Partial<CreateIssueRequest>;
}

function buildMyRelationPlan(
  scope: Extract<IssueScope, { type: "my" }>,
  scopeKey: string,
): IssueSurfaceQueryPlan {
  switch (scope.relation) {
    case "assigned":
      return {
        scopeKey,
        queryFilter: { assignee_id: scope.userId },
        createDefaults: {
          assignee_type: "member",
          assignee_id: scope.userId,
        },
      };
    case "created":
      return {
        scopeKey,
        queryFilter: { creator_id: scope.userId },
        createDefaults: {},
      };
    case "involved":
      return {
        scopeKey,
        queryFilter: { involves_user_id: scope.userId },
        createDefaults: {},
      };
    case "all":
      return { scopeKey, queryFilter: {}, createDefaults: {} };
  }
}

export function buildIssueSurfaceQueryPlan(
  scope: IssueScope,
): IssueSurfaceQueryPlan {
  const scopeKey = issueScopeKey(scope);

  switch (scope.type) {
    case "workspace": {
      const assigneeTypes = assigneeTypesForActorKind(scope.actorKind);
      return {
        scopeKey,
        queryFilter: assigneeTypes ? { assignee_types: assigneeTypes } : {},
        createDefaults: {},
      };
    }
    case "project": {
      const assigneeTypes = assigneeTypesForActorKind(scope.actorKind);
      return {
        scopeKey,
        queryFilter: assigneeTypes
          ? { project_id: scope.projectId, assignee_types: assigneeTypes }
          : { project_id: scope.projectId },
        createDefaults: { project_id: scope.projectId },
      };
    }
    case "my":
      return buildMyRelationPlan(scope, scopeKey);
    case "actor":
      return {
        scopeKey,
        queryFilter:
          scope.relation === "assigned"
            ? { assignee_id: scope.actorId }
            : { creator_id: scope.actorId },
        createDefaults:
          scope.relation === "assigned"
            ? {
                assignee_type: scope.actorType,
                assignee_id: scope.actorId,
              }
            : {},
      };
    case "worklane":
      return {
        scopeKey,
        // Gantt is not offered on a worklane, so there is no list-API filter.
        queryFilter: {},
        createDefaults: {},
      };
    case "team":
      throw new UnsupportedIssueScopeError(scope, "issue surface query plan");
  }
}

/**
 * The scope as the Table channel's `scope` and the filters the scope itself
 * imposes. A worklane's Type filter picks travel as `filters.types` (`none` is
 * No type); no pick means every type, so the filter is left out.
 */
export interface IssueTableScopeQuery {
  scope: IssueTableScope;
  filters: Pick<IssueTableFilters, "types">;
}

export function buildIssueTableScopeQuery(
  scope: IssueScope,
): IssueTableScopeQuery {
  switch (scope.type) {
    case "workspace": {
      const assigneeTypes = assigneeTypesForActorKind(scope.actorKind);
      return {
        scope: {
          kind: "workspace",
          ...(assigneeTypes ? { assignee_types: assigneeTypes } : {}),
        },
        filters: {},
      };
    }
    case "project": {
      const assigneeTypes = assigneeTypesForActorKind(scope.actorKind);
      return {
        scope: {
          kind: "project",
          project_id: scope.projectId,
          ...(assigneeTypes ? { assignee_types: assigneeTypes } : {}),
        },
        filters: {},
      };
    }
    case "my":
      return {
        scope: {
          kind: "my",
          relation: scope.relation === "all" ? "any" : scope.relation,
        },
        filters: {},
      };
    case "actor":
      return {
        scope: {
          kind: scope.relation === "assigned" ? "assignee" : "creator",
          actor: { type: scope.actorType, id: scope.actorId },
        },
        filters: {},
      };
    case "worklane":
      return {
        scope: { kind: "worklane", worklane_id: scope.worklaneId },
        filters:
          scope.issueTypes.length > 0 ? { types: [...scope.issueTypes] } : {},
      };
    case "team":
      throw new UnsupportedIssueScopeError(scope, "the Table query");
  }
}
