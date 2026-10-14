"use client";

import { useMemo } from "react";
import { Shapes } from "lucide-react";
import type { Issue } from "@multica/core/types";
import { ISSUE_TYPE_PICKS, typeCounts, type IssueTypePick } from "@multica/core/worklanes";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { cn } from "@multica/ui/lib/utils";
import { TypeIcon } from "../../issues/components/type-icon";
import { useT } from "../../i18n";

/**
 * The Type filter of a worklane's header: the eight types and No type, each
 * with its icon and how many of the worklane's issues it keeps. Nothing picked
 * shows every type.
 */
export function WorklaneTypeFilter({
  issues,
  value,
  onChange,
}: {
  issues: readonly Issue[];
  value: readonly IssueTypePick[];
  onChange: (types: IssueTypePick[]) => void;
}) {
  const { t } = useT("worklanes");
  const { t: tIssues } = useT("issues");
  const counts = useMemo(() => typeCounts(issues), [issues]);
  const picked = new Set(value);

  const toggle = (pick: IssueTypePick) => {
    onChange(ISSUE_TYPE_PICKS.filter((p) => (p === pick ? !picked.has(p) : picked.has(p))));
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="outline"
            size="sm"
            className={cn("gap-1.5", value.length > 0 && "border-brand text-brand")}
          />
        }
      >
        <Shapes className="size-3.5" aria-hidden="true" />
        {value.length > 0
          ? `${t(($) => $.type_filter.label)} · ${t(($) => $.type_filter.picked, { count: value.length })}`
          : t(($) => $.type_filter.label)}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-auto min-w-48">
        {ISSUE_TYPE_PICKS.map((pick) => (
          <DropdownMenuCheckboxItem
            key={pick}
            checked={picked.has(pick)}
            onCheckedChange={() => toggle(pick)}
          >
            <span aria-hidden="true" className="inline-flex">
              <TypeIcon type={pick === "none" ? null : pick} />
            </span>
            <span className="flex-1">
              {pick === "none"
                ? tIssues(($) => $.types.none)
                : tIssues(($) => $.types[pick].name)}
            </span>
            <span className="text-caption tabular-nums text-muted-foreground">{counts[pick]}</span>
          </DropdownMenuCheckboxItem>
        ))}
        {value.length > 0 && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => onChange([])}>
              {t(($) => $.type_filter.clear)}
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
