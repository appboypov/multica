"use client";

import { toast } from "sonner";
import { MoreHorizontal, X } from "lucide-react";
import type { Issue, Release } from "@multica/core/types";
import { useWorkspacePaths } from "@multica/core/paths";
import { useSetIssuePlacement } from "@multica/core/issues/placement";
import { formatVersion } from "@multica/core/releases";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { AppLink } from "../../navigation";
import { TypeIcon } from "../../issues/components/type-icon";
import { useIssuePeekLinkProps } from "../../issues/surface/peek-context";
import { useT } from "../../i18n";

/**
 * One issue of a worklane in a release column of its Timeline: type icon, ID
 * and title. A click opens the issue's preview inside a peek host, else the
 * issue's page. In an unshipped release its menu offers Remove from release,
 * which keeps the worklane and puts the issue in No release.
 */
export function ReleaseIssueRow({ issue, release }: { issue: Issue; release?: Release }) {
  const paths = useWorkspacePaths();
  const peekLinkProps = useIssuePeekLinkProps(issue.id);
  return (
    <div className="group flex min-w-0 items-center">
      <AppLink
        href={paths.issueDetail(issue.id)}
        newTabTitle={issue.identifier}
        className="flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-body transition-colors hover:bg-accent"
        {...peekLinkProps}
      >
        <TypeIcon type={issue.type ?? null} />
        <span className="shrink-0 text-caption text-muted-foreground">{issue.identifier}</span>
        <span className="min-w-0 truncate">{issue.title}</span>
      </AppLink>
      {release && release.shipped_at === null ? <RemoveFromRelease issue={issue} release={release} /> : null}
    </div>
  );
}

function RemoveFromRelease({ issue, release }: { issue: Issue; release: Release }) {
  const { t } = useT("releases");
  const setPlacement = useSetIssuePlacement();
  const remove = async () => {
    try {
      await setPlacement.mutateAsync({ id: issue.id, worklane_id: issue.worklane_id ?? null, release_id: null });
      toast.success(
        t(($) => $.issues.toast_removed, { identifier: issue.identifier, version: formatVersion(release.version) }),
      );
    } catch {
      toast.error(t(($) => $.issues.remove_failed));
    }
  };
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t(($) => $.issues.row_menu)}
            className="shrink-0 text-muted-foreground opacity-0 group-hover:opacity-100 focus-visible:opacity-100 data-[popup-open]:opacity-100"
          >
            <MoreHorizontal className="size-4" aria-hidden="true" />
          </Button>
        }
      />
      <DropdownMenuContent align="end" className="w-auto min-w-44">
        <DropdownMenuItem disabled={setPlacement.isPending} onClick={() => void remove()}>
          <X className="size-3.5" aria-hidden="true" />
          {t(($) => $.issues.remove)}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
