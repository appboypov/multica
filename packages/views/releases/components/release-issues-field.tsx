"use client";

import { useId } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueStatusCategory } from "@multica/core/issues";
import { useIssueStatuses } from "@multica/core/issue-statuses/hooks";
import { worklaneIssuesOptions } from "@multica/core/issues/placement";
import type { Issue } from "@multica/core/types";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Label } from "@multica/ui/components/ui/label";
import { useT } from "../../i18n";
import { StatusIcon } from "../../issues/components/status-icon";
import { TypeIcon } from "../../issues/components/type-icon";
import { usePlacementData } from "../../issues/components/pickers";

const EMPTY: Issue[] = [];

/**
 * Issues: this worklane's issues in no release ("{worklane} · in no release"),
 * each with its type, ID, title and status, to tick into a release. Used by
 * Plan release and Add issue to release. When every issue of the worklane is
 * in a release it says so; Plan release adds that issues can come later.
 */
export function ReleaseIssuesField({
  worklaneId,
  selected,
  onChange,
  showLater = false,
}: {
  worklaneId: string;
  selected: readonly string[];
  onChange: (ids: string[]) => void;
  /** Plan release: "Add issues later from the timeline." when none is listed. */
  showLater?: boolean;
}) {
  const { t } = useT("releases");
  const labelId = useId();
  const wsId = useWorkspaceId();
  const { data: issues = EMPTY, isLoading } = useQuery(worklaneIssuesOptions(wsId, worklaneId));
  const { colorOf, iconOf } = useIssueStatuses(wsId);
  const worklane = usePlacementData().worklanes.find((w) => w.id === worklaneId);
  const free = issues.filter((issue) => !issue.release_id);

  const toggle = (id: string, checked: boolean) =>
    onChange(checked ? [...selected, id] : selected.filter((other) => other !== id));

  return (
    <div role="group" aria-labelledby={labelId} className="space-y-1">
      <span id={labelId} className="flex flex-col text-caption text-muted-foreground">
        <span>{t(($) => $.issues.field_label)}</span>
        {worklane ? <span>{t(($) => $.issues.field_from, { worklane: worklane.name })}</span> : null}
      </span>
      {isLoading ? null : free.length === 0 ? (
        <p className="rounded-md border border-dashed px-3 py-2 text-caption text-muted-foreground">
          {showLater
            ? `${t(($) => $.issues.field_empty)} ${t(($) => $.issues.field_later)}`
            : t(($) => $.issues.field_empty)}
        </p>
      ) : (
        <ul className="max-h-56 overflow-y-auto rounded-md border">
          {free.map((issue) => {
            const id = `${labelId}-${issue.id}`;
            return (
              <li key={issue.id} className="flex items-center gap-2 px-3 py-1.5 hover:bg-accent/50">
                <Checkbox
                  id={id}
                  checked={selected.includes(issue.id)}
                  onCheckedChange={(checked) => toggle(issue.id, checked === true)}
                />
                <Label htmlFor={id} className="flex min-w-0 flex-1 items-center gap-2 font-normal">
                  <TypeIcon type={issue.type ?? null} />
                  <span className="shrink-0 text-caption text-muted-foreground">{issue.identifier}</span>
                  <span className="min-w-0 flex-1 truncate text-body">{issue.title}</span>
                  <StatusIcon
                    status={issue.status}
                    color={colorOf(issue.status)}
                    icon={iconOf(issue.status)}
                    category={issueStatusCategory(issue) ?? undefined}
                    className="size-3.5 shrink-0"
                  />
                </Label>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
