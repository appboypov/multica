"use client";

import type { Worklane, WorklaneIssueRef, WorklaneState } from "@multica/core/types";
import { useWorkspacePaths } from "@multica/core/paths";
import { cn } from "@multica/ui/lib/utils";
import { AppLink } from "../../navigation";
import { useIssuePeekLinkProps } from "../../issues/surface/peek-context";
import { useT } from "../../i18n";

/** A worklane's state as a dot and a word: Free, or Busy while an issue of it is worked on. */
export function WorklaneStateLabel({
  state,
  className,
}: {
  state: WorklaneState;
  className?: string;
}) {
  const { t } = useT("worklanes");
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1.5 text-caption",
        state === "busy" ? "text-foreground" : "text-muted-foreground",
        className,
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "size-1.5 rounded-full",
          state === "busy" ? "bg-warning" : "bg-success",
        )}
      />
      {t(($) => $.state[state])}
    </span>
  );
}

/**
 * A worklane's header state: Busy or Free, then "now: {ID} {title}" for the
 * issue that holds its worktree's branch, or "next: {ID} {title}" for the
 * issue it takes next; neither once every issue is Done or Cancelled. The
 * issue opens its preview inside a peek host.
 */
export function WorklaneHead({ worklane }: { worklane: Worklane }) {
  const { t } = useT("worklanes");
  const issue = worklane.state === "busy" ? worklane.active_issue : worklane.next_issue;
  return (
    <span className="inline-flex min-w-0 items-center gap-3">
      <WorklaneStateLabel state={worklane.state} />
      {issue && (
        <span className="inline-flex min-w-0 items-center gap-1.5 text-caption text-muted-foreground">
          <span className="shrink-0">
            {worklane.state === "busy" ? t(($) => $.head.now) : t(($) => $.head.next)}
          </span>
          <WorklaneHeadIssue issue={issue} />
        </span>
      )}
    </span>
  );
}

function WorklaneHeadIssue({ issue }: { issue: WorklaneIssueRef }) {
  const { t } = useT("worklanes");
  const paths = useWorkspacePaths();
  const peekLinkProps = useIssuePeekLinkProps(issue.id);
  return (
    <AppLink
      href={paths.issueDetail(issue.id)}
      newTabTitle={issue.identifier}
      title={t(($) => $.head.open, { identifier: issue.identifier })}
      className="inline-flex min-w-0 items-center gap-1.5 text-foreground hover:underline"
      {...peekLinkProps}
    >
      <span className="shrink-0 font-mono">{issue.identifier}</span>
      <span className="truncate">{issue.title}</span>
    </AppLink>
  );
}
