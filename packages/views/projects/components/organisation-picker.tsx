"use client";

import { Building2, Check } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useOrganisations } from "@multica/core/organisations";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { cn } from "@multica/ui/lib/utils";
import { PillButton } from "../../common/pill-button";
import { useT } from "../../i18n";

/**
 * The one organisation control of New project and the project page: a pill
 * showing the project's organisation, or "No organisation" when it has none
 * or points to one the workspace no longer lists, and a menu of the
 * workspace's organisations, which the server returns by name.
 */
export function OrganisationPicker({
  organisationId,
  onChange,
  align = "start",
  className,
}: {
  organisationId: string | null | undefined;
  onChange: (organisationId: string) => void;
  align?: "start" | "end" | "center";
  className?: string;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const { data: organisations = [] } = useOrganisations(wsId);
  const selected = organisationId
    ? organisations.find((organisation) => organisation.id === organisationId)
    : undefined;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <PillButton className={className}>
            <Building2 className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className={cn("truncate", !selected && "text-muted-foreground")}>
              {selected?.name ?? t(($) => $.organisation.none)}
            </span>
          </PillButton>
        }
      />
      <DropdownMenuContent align={align} className="max-h-72 w-56 overflow-y-auto">
        {organisations.length === 0 ? (
          <DropdownMenuItem disabled>{t(($) => $.organisation.empty)}</DropdownMenuItem>
        ) : (
          organisations.map((organisation) => {
            const isSelected = organisation.id === selected?.id;
            return (
              <DropdownMenuItem
                key={organisation.id}
                onClick={() => {
                  if (!isSelected) onChange(organisation.id);
                }}
              >
                <span className="min-w-0 flex-1 truncate">{organisation.name}</span>
                {isSelected && <Check className="size-3.5 shrink-0" />}
              </DropdownMenuItem>
            );
          })
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
