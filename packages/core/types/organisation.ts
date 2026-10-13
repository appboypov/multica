/**
 * Organisations — workspace-scoped holders of projects (ADR-0004).
 *
 * A name is unique in its workspace, ignoring case. A key is unique in its
 * workspace and names the organisation's folder under `~/Worktrees`, so it is
 * lowercase letters, digits and dashes, starting with a letter or digit.
 * Organisations are never renamed, so an update carries only the key.
 */
export interface Organisation {
  id: string;
  workspace_id: string;
  name: string;
  key: string;
  created_at: string;
  updated_at: string;
}

export interface CreateOrganisationRequest {
  name: string;
  key: string;
}

export interface UpdateOrganisationRequest {
  key: string;
}

export interface ListOrganisationsResponse {
  organisations: Organisation[];
  total: number;
}
