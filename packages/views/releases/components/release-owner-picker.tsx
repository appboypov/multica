"use client";

import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Lock, UserMinus } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { canAssignAgentToIssue, useCurrentMember } from "@multica/core/permissions";
import type { ReleaseOwnerRequest, ReleaseOwnerType } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { agentListOptions, squadListOptions } from "@multica/core/workspace/queries";
import { cn } from "@multica/ui/lib/utils";
import { Spinner } from "@multica/ui/components/ui/spinner";
import { ActorAvatar } from "../../common/actor-avatar";
import { matchesPinyin } from "../../editor/extensions/pinyin-match";
import {
  PickerEmpty,
  PickerItem,
  PickerSection,
  PropertyPicker,
} from "../../issues/components/pickers/property-picker";
import { useT } from "../../i18n";

export const NO_OWNER: ReleaseOwnerRequest = { owner_type: null, owner_id: null };

/**
 * The owner of a release as it reads: its avatar and name, or "No owner".
 * Used by the picker's trigger and wherever an owner shows without one.
 */
export function ReleaseOwnerLabel({
  ownerType,
  ownerId,
  ownerName,
  className,
}: {
  ownerType: ReleaseOwnerType | null;
  ownerId: string | null;
  /** The name the server sent with the release; the directory names it otherwise. */
  ownerName?: string | null;
  className?: string;
}) {
  const { t } = useT("releases");
  const { getActorName } = useActorName();
  if (!ownerType || !ownerId) {
    return <span className={cn("truncate text-muted-foreground", className)}>{t(($) => $.owner.none)}</span>;
  }
  const name = ownerName ?? getActorName(ownerType, ownerId);
  return (
    <span className={cn("flex min-w-0 items-center gap-1.5", className)}>
      <ActorAvatar actorType={ownerType} actorId={ownerId} name={name} size="sm" />
      <span className="truncate">{name}</span>
    </span>
  );
}

/**
 * The Owner picker of Plan release, Edit release and the release page: "No
 * owner" first, then the workspace's agents and squads, never members. An
 * agent the current member could not assign an issue to is shown disabled
 * with the reason; an owner without a runtime is allowed and just gets no run.
 */
export function ReleaseOwnerPicker({
  ownerType,
  ownerId,
  ownerName,
  onChange,
  labelledBy,
  variant,
  disabled = false,
  pending = false,
}: {
  ownerType: ReleaseOwnerType | null;
  ownerId: string | null;
  ownerName?: string | null;
  onChange: (owner: ReleaseOwnerRequest) => void;
  /** Id of the visible "Owner" label, so the trigger reads "Owner, Pieter". */
  labelledBy: string;
  /** A form field in a dialog, or inline on the release page. */
  variant: "field" | "inline";
  disabled?: boolean;
  /** The last pick is being saved. */
  pending?: boolean;
}) {
  const { t } = useT("releases");
  const wsId = useWorkspaceId();
  const valueId = useId();
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const { userId, role } = useCurrentMember(wsId);
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: squads = [] } = useQuery(squadListOptions(wsId));

  const query = filter.trim().toLowerCase();
  const shownAgents = agents.filter(
    (agent) => !agent.archived_at && (agent.name.toLowerCase().includes(query) || matchesPinyin(agent.name, query)),
  );
  const shownSquads = squads.filter(
    (squad) => !squad.archived_at && (squad.name.toLowerCase().includes(query) || matchesPinyin(squad.name, query)),
  );

  const pick = (owner: ReleaseOwnerRequest) => {
    setOpen(false);
    if (owner.owner_type === ownerType && owner.owner_id === ownerId) return;
    onChange(owner);
  };

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-64"
      align="start"
      searchable
      searchPlaceholder={t(($) => $.owner.search)}
      onSearchChange={setFilter}
      triggerRender={
        <button
          type="button"
          disabled={disabled || pending}
          aria-busy={pending || undefined}
          aria-labelledby={`${labelledBy} ${valueId}`}
          className={cn(
            "flex min-w-0 items-center gap-2 text-left text-body transition-colors disabled:cursor-default",
            variant === "field"
              ? "h-9 w-full rounded-md border bg-background px-3 hover:bg-accent/40 disabled:opacity-60"
              : "-mx-1 max-w-full cursor-pointer rounded-xs px-1 py-0.5 hover:bg-accent/40",
          )}
        />
      }
      trigger={
        <>
          <span id={valueId} className="flex min-w-0 flex-1 items-center">
            <ReleaseOwnerLabel ownerType={ownerType} ownerId={ownerId} ownerName={ownerName} />
          </span>
          {pending ? <Spinner className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" /> : null}
        </>
      }
    >
      <PickerItem emptyValue selected={!ownerType || !ownerId} onClick={() => pick(NO_OWNER)}>
        <UserMinus className="size-3.5 text-muted-foreground" aria-hidden="true" />
        <span className="text-muted-foreground">{t(($) => $.owner.none)}</span>
      </PickerItem>

      {shownAgents.length > 0 ? (
        <PickerSection label={t(($) => $.owner.agents)}>
          {shownAgents.map((agent) => {
            const decision = canAssignAgentToIssue(agent, { userId, role });
            return (
              <PickerItem
                key={agent.id}
                selected={ownerType === "agent" && ownerId === agent.id}
                disabled={!decision.allowed}
                tooltip={decision.allowed ? undefined : decision.message}
                onClick={() => {
                  if (decision.allowed) pick({ owner_type: "agent", owner_id: agent.id });
                }}
              >
                <ActorAvatar actorType="agent" actorId={agent.id} name={agent.name} size="sm" />
                <span className={cn("truncate", !decision.allowed && "text-muted-foreground")}>{agent.name}</span>
                {agent.visibility === "private" ? (
                  <Lock className="ml-auto size-3 text-muted-foreground" aria-hidden="true" />
                ) : null}
              </PickerItem>
            );
          })}
        </PickerSection>
      ) : null}

      {shownSquads.length > 0 ? (
        <PickerSection label={t(($) => $.owner.squads)}>
          {shownSquads.map((squad) => (
            <PickerItem
              key={squad.id}
              selected={ownerType === "squad" && ownerId === squad.id}
              onClick={() => pick({ owner_type: "squad", owner_id: squad.id })}
            >
              <ActorAvatar actorType="squad" actorId={squad.id} name={squad.name} size="sm" />
              <span className="truncate">{squad.name}</span>
            </PickerItem>
          ))}
        </PickerSection>
      ) : null}

      {shownAgents.length === 0 && shownSquads.length === 0 && query ? <PickerEmpty /> : null}
    </PropertyPicker>
  );
}
