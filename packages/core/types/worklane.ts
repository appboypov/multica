/**
 * Worklanes — the lanes of work of a repository (ADR-0006).
 *
 * Every repository has its `main` worklane, made with it: `is_main` true, no
 * `folder` and no `daemon_id`, because it works in the repository's checkout.
 * Names are unique per repository, ignoring case.
 */
export interface Worklane {
  id: string;
  workspace_id: string;
  repository_id: string;
  name: string;
  /** The worktree folder under the repository's Worktrees folder; null for `main`. */
  folder: string | null;
  /** The machine of the worktree; null for `main`. */
  daemon_id: string | null;
  is_main: boolean;
  position: number;
  created_at: string;
  updated_at: string;
}

export interface ListWorklanesResponse {
  worklanes: Worklane[];
  total: number;
}
