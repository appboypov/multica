"use client";

import type { Issue } from "@multica/core/types";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../../navigation";
import { TypeIcon } from "../../issues/components/type-icon";
import { useIssuePeekLinkProps } from "../../issues/surface/peek-context";

/**
 * One issue of a worklane in a release column of its Timeline: type icon, ID
 * and title. A click opens the issue's preview inside a peek host, else the
 * issue's page.
 */
export function ReleaseIssueRow({ issue }: { issue: Issue }) {
  const paths = useWorkspacePaths();
  const peekLinkProps = useIssuePeekLinkProps(issue.id);
  return (
    <AppLink
      href={paths.issueDetail(issue.id)}
      newTabTitle={issue.identifier}
      className="flex min-w-0 items-center gap-2 rounded-md px-2 py-1.5 text-body transition-colors hover:bg-accent"
      {...peekLinkProps}
    >
      <TypeIcon type={issue.type ?? null} />
      <span className="shrink-0 text-caption text-muted-foreground">{issue.identifier}</span>
      <span className="min-w-0 truncate">{issue.title}</span>
    </AppLink>
  );
}
