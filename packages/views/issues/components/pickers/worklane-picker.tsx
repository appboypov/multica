"use client";

import { useMemo, useState } from "react";
import { GitBranch } from "lucide-react";
import { worklaneOverview } from "@multica/core/worklanes";
import { PropertyPicker, PickerItem, PickerSection, PickerEmpty } from "./property-picker";
import { usePlacementData } from "./use-placement-data";
import { useT } from "../../../i18n";

const PICKER_ORDER = { field: "name", direction: "asc" } as const;

interface WorklanePickerProps {
  worklaneId: string | null;
  onPick: (worklaneId: string | null) => void;
  align?: "start" | "center" | "end";
  /** Overrides the default trigger (icon, worklane and repository). */
  trigger?: React.ReactNode;
  /** The element the trigger renders as, such as New issue's pill. */
  triggerRender?: React.ReactElement;
}

/**
 * Worklane in New issue and on the issue page: every worklane of the
 * workspace grouped by organisation, project and repository, with
 * "No worklane" first.
 */
export function WorklanePicker({
  worklaneId,
  onPick,
  align,
  trigger,
  triggerRender,
}: WorklanePickerProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const { t } = useT("issues");
  const data = usePlacementData();

  const tree = useMemo(() => worklaneOverview({ ...data, order: PICKER_ORDER }), [data]);
  const picked = worklaneId ? data.worklanes.find((w) => w.id === worklaneId) : undefined;
  const pickedRepository = picked
    ? data.repositories.find((r) => r.id === picked.repository_id)
    : undefined;

  const needle = query.trim().toLowerCase();
  const matches = (...texts: string[]) =>
    needle === "" || texts.some((text) => text.toLowerCase().includes(needle));

  const pick = (id: string | null) => {
    onPick(id);
    setOpen(false);
  };

  const groups = tree.groups
    .map((group) => ({
      key: group.organisation?.id ?? "none",
      label: group.organisation?.name ?? t(($) => $.pickers.worklane.no_organisation),
      repositories: group.projects.flatMap(({ project, repositories }) =>
        repositories
          .map(({ repository, worklanes }) => ({
            project,
            repository,
            worklanes: worklanes.filter((w) =>
              matches(w.name, repository.full_name, project.title),
            ),
          }))
          .filter((entry) => entry.worklanes.length > 0),
      ),
    }))
    .filter((group) => group.repositories.length > 0);

  const noneLabel = t(($) => $.pickers.worklane.none);

  return (
    <PropertyPicker
      open={open}
      onOpenChange={(v) => {
        setOpen(v);
        if (!v) setQuery("");
      }}
      width="w-80"
      align={align}
      triggerRender={triggerRender}
      searchable
      searchPlaceholder={t(($) => $.pickers.worklane.search_placeholder)}
      onSearchChange={setQuery}
      trigger={
        trigger ?? (
          <>
            <GitBranch className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <span className="truncate">{picked ? picked.name : noneLabel}</span>
            {pickedRepository && (
              <span className="truncate font-mono text-caption text-muted-foreground">
                {pickedRepository.full_name}
              </span>
            )}
          </>
        )
      }
    >
      {matches(noneLabel) && (
        <PickerItem selected={worklaneId === null} emptyValue onClick={() => pick(null)}>
          <GitBranch className="h-3.5 w-3.5 shrink-0 text-faint-foreground" />
          <span className="truncate text-muted-foreground">{noneLabel}</span>
        </PickerItem>
      )}
      {groups.map((group) => (
        <PickerSection key={group.key} label={group.label}>
          {group.repositories.map(({ project, repository, worklanes }) => (
            <div key={repository.id}>
              <div className="flex min-w-0 items-center gap-1.5 px-2 pt-1.5 pb-0.5 text-caption text-muted-foreground">
                <span className="truncate">{project.title}</span>
                <span className="truncate font-mono">{repository.full_name}</span>
              </div>
              {worklanes.map((worklane) => (
                <PickerItem
                  key={worklane.id}
                  selected={worklane.id === worklaneId}
                  onClick={() => pick(worklane.id)}
                >
                  <GitBranch className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  <span className="truncate">{worklane.name}</span>
                </PickerItem>
              ))}
            </div>
          ))}
        </PickerSection>
      ))}
      {groups.length === 0 && !matches(noneLabel) && <PickerEmpty />}
    </PropertyPicker>
  );
}
