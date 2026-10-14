"use client";

import { useWorkspaceId } from "@multica/core/hooks";
import { useIssueStatuses } from "@multica/core/issue-statuses/hooks";
import type { Worklane } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { StatusIcon } from "../../issues/components/status-icon";

/**
 * The line under a worklane's name on every worklane row: the issue it works
 * on while Busy, the issue it takes next while Free, else "Nothing queued".
 * The server picks both issues; the row only shows them.
 */
export function WorklaneRowWork({
  worklane,
  className,
}: {
  worklane: Pick<Worklane, "state" | "active_issue" | "next_issue">;
  className?: string;
}) {
  const { t } = useT("worklanes");
  const wsId = useWorkspaceId();
  const statuses = useIssueStatuses(wsId);
  const busy = worklane.state === "busy";
  const issue = busy ? worklane.active_issue : worklane.next_issue;

  if (!issue) {
    return (
      <span className={cn("block truncate text-caption text-muted-foreground", className)}>
        {t(($) => $.row_work.nothing_queued)}
      </span>
    );
  }

  const statusLabel = statuses.labelOf(issue.status);
  return (
    <span className={cn("flex min-w-0 items-center gap-1.5 text-caption", className)}>
      <span role="img" aria-label={statusLabel} title={statusLabel} className="inline-flex shrink-0">
        <StatusIcon
          status={issue.status}
          category={statuses.categoryOf(issue.status)}
          color={statuses.colorOf(issue.status)}
          icon={statuses.iconOf(issue.status)}
          className="h-3.5 w-3.5"
        />
      </span>
      <span className="min-w-0 truncate">
        {!busy && (
          <span className="text-muted-foreground">{t(($) => $.row_work.next)} </span>
        )}
        <span className="font-medium text-muted-foreground">{issue.identifier}</span>{" "}
        <span className="text-foreground">{issue.title}</span>
      </span>
    </span>
  );
}
