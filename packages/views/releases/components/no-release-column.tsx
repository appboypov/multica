"use client";

import type { Issue } from "@multica/core/types";
import { useT } from "../../i18n";
import { ReleaseIssueRow } from "./release-issue-row";

/**
 * "No release" after a worklane's release line: the worklane's issues that
 * are in no release, with their count. It also stands alone for a repository
 * that does not release.
 */
export function NoReleaseColumn({ issues }: { issues: readonly Issue[] }) {
  const { t } = useT("releases");
  return (
    <div className="w-[34rem] shrink-0">
      <div className="mb-5 h-3" aria-hidden="true" />
      <section
        aria-label={t(($) => $.issues.no_release)}
        className="rounded-lg border border-dashed bg-card/50"
      >
        <header className="flex items-center gap-2 border-b border-dashed px-4 py-3">
          <h3 className="text-body font-medium">{t(($) => $.issues.no_release)}</h3>
          <span className="text-caption tabular-nums text-muted-foreground">{issues.length}</span>
        </header>
        <div className="flex flex-col py-1">
          {issues.length === 0 ? (
            <p className="px-4 py-3 text-label text-muted-foreground">{t(($) => $.column.no_issues)}</p>
          ) : (
            issues.map((issue) => <ReleaseIssueRow key={issue.id} issue={issue} />)
          )}
        </div>
      </section>
    </div>
  );
}
