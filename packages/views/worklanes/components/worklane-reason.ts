"use client";

import { useCallback } from "react";
import type { WorklaneRequestReason } from "@multica/core/types";
import { useT } from "../../i18n";

/** What a reason's sentence can name besides the server's detail. */
export interface WorklaneReasonContext {
  /** `owner/name` of the repository. */
  repository: string;
  /** The worklane's name. */
  name: string;
  /** The machine the request runs on. */
  runtime: string;
  /** `origin/{default branch}`. */
  base: string;
}

/**
 * Words a refusal or failure the server sent. A code that names something
 * (a taken name, an offline machine) reads it from the server's detail; a
 * code the daemon explains in full (a path in the way, a newer server's
 * code) shows the detail as it is.
 */
export function useWorklaneReasonText() {
  const { t } = useT("worklanes");
  return useCallback(
    (reason: WorklaneRequestReason, context: WorklaneReasonContext): string => {
      const detail = reason.detail;
      switch (reason.code) {
        case "name_taken":
          return t(($) => $.errors.name_taken, {
            repository: context.repository,
            name: detail || context.name,
          });
        case "folder_taken":
          return t(($) => $.errors.folder_taken, { name: detail });
        case "runtime_offline":
        case "runtime_cannot_make_worktrees":
          return t(($) => $.errors[reason.code], { name: detail || context.runtime });
        case "runtime_no_checkout":
          return t(($) => $.errors.runtime_no_checkout, { repository: context.repository });
        case "lost":
          return t(($) => $.errors.lost, { runtime: context.runtime });
        case "busy":
        case "uncommitted":
        case "stashed":
          return t(($) => $.errors[reason.code], { name: context.name });
        case "unpushed":
          return t(($) => $.errors.unpushed, { name: context.name, base: context.base });
        case "worklane_has_open_issues":
          // The server's detail ends with the identifiers of the open issues.
          return t(($) => $.errors.worklane_has_open_issues, {
            name: context.name,
            issues: detail.replace(/^.*open issues\s*/, ""),
          });
        case "path_issue_worktree":
        case "path_not_checkout":
        case "unknown":
          return t(($) => $.errors[reason.code], { detail });
        default:
          return t(($) => $.errors[reason.code]);
      }
    },
    [t],
  );
}
