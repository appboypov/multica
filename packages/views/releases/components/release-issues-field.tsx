"use client";

import { useId } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { worklaneIssuesOptions } from "@multica/core/issues/placement";
import type { Issue } from "@multica/core/types";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Label } from "@multica/ui/components/ui/label";
import { useT } from "../../i18n";
import { TypeIcon } from "../../issues/components/type-icon";

const EMPTY: Issue[] = [];

/**
 * Issues: this worklane's issues in no release, to tick into a release. Used
 * by Plan release and Add issue to release. When every issue of the worklane
 * is in a release it says so; Plan release adds that issues can come later.
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
  /** Plan release: "Add issues later from the timeline." under the list. */
  showLater?: boolean;
}) {
  const { t } = useT("releases");
  const labelId = useId();
  const wsId = useWorkspaceId();
  const { data: issues = EMPTY, isLoading } = useQuery(worklaneIssuesOptions(wsId, worklaneId));
  const free = issues.filter((issue) => !issue.release_id);

  const toggle = (id: string, checked: boolean) =>
    onChange(checked ? [...selected, id] : selected.filter((other) => other !== id));

  return (
    <div role="group" aria-labelledby={labelId} className="space-y-1">
      <span id={labelId} className="text-caption text-muted-foreground">
        {t(($) => $.issues.field_label)}
      </span>
      {isLoading ? null : free.length === 0 ? (
        <p className="rounded-md border border-dashed px-3 py-2 text-caption text-muted-foreground">
          {t(($) => $.issues.field_empty)}
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
                  <span className="min-w-0 truncate text-body">{issue.title}</span>
                </Label>
              </li>
            );
          })}
        </ul>
      )}
      {showLater ? <p className="text-caption text-muted-foreground">{t(($) => $.issues.field_later)}</p> : null}
    </div>
  );
}
