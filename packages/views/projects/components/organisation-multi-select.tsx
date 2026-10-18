"use client";

import { useState } from "react";
import { Building2, ChevronDown, X } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useOrganisations } from "@multica/core/organisations";
import { cn } from "@multica/ui/lib/utils";
import {
  PropertyPicker,
  PickerItem,
  PickerEmpty,
} from "../../issues/components/pickers/property-picker";
import { matchesPinyin } from "../../editor/extensions/pinyin-match";
import { useT } from "../../i18n";

/**
 * A field of chosen organisations: each one a chip with a remove button,
 * followed by a searchable menu of the workspace's organisations. Fully
 * controlled, the parent owns the selection.
 */
export function OrganisationMultiSelect({
  selectedIds,
  onChange,
  id,
}: {
  selectedIds: readonly string[];
  onChange: (next: string[]) => void;
  id?: string;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const { data: organisations = [] } = useOrganisations(wsId);
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");

  const query = filter.trim().toLowerCase();
  const matching = organisations.filter(
    (o) => o.name.toLowerCase().includes(query) || matchesPinyin(o.name, query),
  );
  const selected = organisations.filter((o) => selectedIds.includes(o.id));

  const toggle = (organisationId: string) => {
    onChange(
      selectedIds.includes(organisationId)
        ? selectedIds.filter((s) => s !== organisationId)
        : [...selectedIds, organisationId],
    );
  };

  return (
    <div className="flex min-h-9 w-full flex-wrap items-center gap-1 rounded-md border border-input bg-transparent px-1 py-1 dark:bg-input/30">
      {selected.map((o) => (
        <span
          key={o.id}
          className="inline-flex max-w-full items-center gap-1 rounded-sm bg-muted py-0.5 pl-2 pr-1 text-caption font-medium"
        >
          <span className="truncate">{o.name}</span>
          <button
            type="button"
            onClick={() => toggle(o.id)}
            className="rounded-xs p-0.5 text-muted-foreground transition-colors hover:text-foreground cursor-pointer"
            aria-label={t(($) => $.organisation.remove_aria, { name: o.name })}
          >
            <X className="size-3" />
          </button>
        </span>
      ))}
      <PropertyPicker
        open={open}
        onOpenChange={(v) => {
          setOpen(v);
          if (!v) setFilter("");
        }}
        width="w-64"
        align="end"
        searchable
        searchPlaceholder={t(($) => $.organisation.search_placeholder)}
        onSearchChange={setFilter}
        triggerRender={
          <button
            type="button"
            id={id}
            className="flex min-h-7 min-w-16 flex-1 cursor-pointer items-center justify-between gap-2 rounded-sm px-1.5 text-left text-body outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
          />
        }
        trigger={
          <>
            <span className={cn("truncate text-muted-foreground", selected.length > 0 && "sr-only")}>
              {t(($) => $.organisation.placeholder)}
            </span>
            <ChevronDown className="ml-auto size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          </>
        }
      >
        {organisations.length === 0 ? (
          <div className="px-2 py-3 text-center text-body text-muted-foreground">
            {t(($) => $.organisation.empty)}
          </div>
        ) : matching.length === 0 ? (
          <PickerEmpty />
        ) : (
          matching.map((o) => (
            <PickerItem key={o.id} selected={selectedIds.includes(o.id)} onClick={() => toggle(o.id)}>
              <Building2 className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
              <span className="truncate">{o.name}</span>
            </PickerItem>
          ))
        )}
      </PropertyPicker>
    </div>
  );
}
