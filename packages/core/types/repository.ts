/**
 * Repositories — a GitHub repository of a project, with its normal checkout on
 * one machine (ADR-0006).
 *
 * A repository is in at most one project of the workspace. Its GitHub fields
 * are read from GitHub on create and on every edit, and `full_name` never
 * changes: an edit moves only the checkout and the version policy.
 */

/**
 * Where a repository's version comes from: one of four version files, its
 * `v{version}` tags only, or `none` for "This repository does not release".
 */
export type VersionPolicy =
  | "pubspec.yaml"
  | "Cargo.toml"
  | "pyproject.toml"
  | "package.json"
  | "tags"
  | "none";

export interface Repository {
  id: string;
  workspace_id: string;
  project_id: string;
  created_by: string | null;
  created_at: string;
  updated_at: string;
  github_repo_id: number;
  /** `owner/name` as GitHub returns it. */
  full_name: string;
  private: boolean;
  default_branch: string;
  /** The machine of the normal checkout. */
  checkout_daemon_id: string;
  /** The absolute path of the normal checkout, as the daemon resolved it. */
  checkout_path: string;
  version_policy: VersionPolicy;
  /**
   * The command that installs the dependencies of a new worktree: null while
   * not found yet, "" for None, anything else is the command.
   */
  install_command: string | null;
  /** null while not known, "" for no tag, otherwise a version without `v`. */
  last_tag: string | null;
  /**
   * "Work in the project folder": every issue task of the repository works in
   * its checkout instead of a worktree (ADR-0031).
   */
  work_in_checkout: boolean;
}

export interface ListRepositoriesResponse {
  repositories: Repository[];
  total: number;
}

/** A repository a GitHub App installation of the workspace can see. */
export interface AvailableRepository {
  github_repo_id: number;
  full_name: string;
  private: boolean;
  default_branch: string;
}

export interface ListAvailableRepositoriesResponse {
  repositories: AvailableRepository[];
  /** The App's installation page, where Brian gives Multica access to more. */
  install_url: string;
}

export interface CreateRepositoryRequest {
  project_id: string;
  full_name: string;
  /** A completed, passed checkout check for the same `full_name`. */
  checkout_check_id: string;
  version_policy: VersionPolicy;
  /** "" means None. Omit it to use the check's first found command. */
  install_command?: string;
}

/** Omit `checkout_check_id` to keep the checkout. */
export interface UpdateRepositoryRequest {
  checkout_check_id?: string;
  version_policy?: VersionPolicy;
  install_command?: string;
  work_in_checkout?: boolean;
}

/** Starts a checkout check, or a clone then the check, on a runtime. */
export interface CreateRepositoryCheckoutRequest {
  full_name: string;
  default_branch: string;
  path: string;
  clone: boolean;
}

export type RepositoryCheckoutStatus = "pending" | "running" | "completed" | "failed" | "timeout";

/** Why a checkout did not pass. An unknown code from a newer server is `unknown`. */
export type RepositoryCheckoutFailureCode =
  | "not_found"
  | "not_git"
  | "origin_mismatch"
  | "wrong_branch"
  | "clone_failed"
  | "unknown";

export interface RepositoryCheckoutFailure {
  code: RepositoryCheckoutFailureCode;
  detail: string;
}

/** What the daemon found at the path. */
export interface RepositoryCheckoutResult {
  /** The path made absolute and clean. */
  resolved_path: string;
  passed: boolean;
  failures: RepositoryCheckoutFailure[];
  /** The version files on `refs/heads/{default_branch}`, in policy order. */
  version_files: string[];
  /** The highest local `v{major}.{minor}.{patch}` tag, or "" for none. */
  last_tag: string;
  /** The install commands of the files found on the default branch, in table order. */
  install_commands: RepositoryInstallCommand[];
}

export interface RepositoryInstallCommand {
  file: string;
  command: string;
}

/** A checkout check as the UI polls it, like a runtime directory listing. */
export interface RepositoryCheckoutRequest extends RepositoryCheckoutResult {
  id: string;
  runtime_id: string;
  status: RepositoryCheckoutStatus;
  error: string;
  created_at: string;
  updated_at: string;
}

/** A repository cannot stop releasing while its line has unshipped releases. */
export interface RepositoryRefusal {
  code: "has_unshipped_releases" | "has_worklanes" | "unknown";
  error?: string;
  releases: { version: string; title: string }[];
  worklanes?: string[];
}
