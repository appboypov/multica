"use client";

import { useState } from "react";
import { Loader2, Plus, RefreshCw, Search, Server, Trash2 } from "lucide-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type { RuntimeLocalSkillSummary, Squad } from "@multica/core/types";
import { api, ApiError } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  isRuntimeUsableForUser,
  runtimeCapabilitiesOptions,
  runtimeDisplayLabel,
  runtimeListOptions,
  runtimeSkillControlsSupported,
} from "@multica/core/runtimes";
import {
  skillDetailOptions,
  skillListOptions,
  squadDetailOptions,
  squadSkillListOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { Switch } from "@multica/ui/components/ui/switch";
import { cn } from "@multica/ui/lib/utils";
import { AssignSkillsDialog } from "../../agents/components/skill-add-dialog";
import {
  CapabilitySection,
  EmptyState,
  RuntimeNotice,
  SkillDetailDialog,
  isRuntimeSkillException,
  runtimeSkillIdentity,
  type SelectedSkill,
} from "../../agents/components/tabs/skill-tab-parts";
import { ProviderLogo } from "../../runtimes/components/provider-logo";
import { SkillIcon } from "../../skills/lib/skill-icon";
import { filterBySkillSearch } from "../../skills/lib/skill-search";
import { HighlightText } from "../../search/highlight-text";
import { useT } from "../../i18n";

/**
 * Skills tab of the squad page. Mirrors the agent Skills tab: assigned
 * workspace skills with an enabled flag, plus runtime skills with a section
 * switch and per-skill switches. A squad has no runtime of its own, so the
 * user picks one of the online local runtimes whose runs enforce skill
 * filters; the squad's runtime skill exceptions are stored per runtime.
 */
export function SquadSkillsTab({
  squad,
  currentUserId,
  canEdit = true,
}: {
  squad: Squad;
  currentUserId?: string | null;
  canEdit?: boolean;
}) {
  const { t } = useT("squads");
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  const { data: workspaceSkills = [] } = useQuery(skillListOptions(wsId));
  const { data: assignedSkills = [] } = useQuery(squadSkillListOptions(wsId, squad.id));
  const { data: runtimes = [] } = useQuery(runtimeListOptions(wsId));
  const viewerId = currentUserId ?? null;
  const switchableRuntimes = runtimes.filter(
    (runtime) =>
      runtime.runtime_mode === "local" &&
      runtime.status === "online" &&
      runtimeSkillControlsSupported(runtime.provider),
  );
  const [pickedRuntimeId, setPickedRuntimeId] = useState<string | null>(null);
  // The picked runtime while it stays switchable, else the first one the
  // viewer may read.
  const runtime =
    switchableRuntimes.find((r) => r.id === pickedRuntimeId) ??
    switchableRuntimes.find((r) => isRuntimeUsableForUser(r, viewerId)) ??
    switchableRuntimes[0] ??
    null;
  const canReadRuntime = runtime != null && isRuntimeUsableForUser(runtime, viewerId);
  const runtimeId = canReadRuntime ? runtime.id : null;
  const runtimeQuery = useQuery(runtimeCapabilitiesOptions(runtimeId));
  const [busyId, setBusyId] = useState<string | null>(null);
  const [showAdd, setShowAdd] = useState(false);
  const [selected, setSelected] = useState<SelectedSkill>(null);
  const [query, setQuery] = useState("");
  const selectedWorkspaceId = selected?.kind === "workspace" ? selected.id : "";
  const detailQuery = useQuery(skillDetailOptions(wsId, selectedWorkspaceId));

  const refreshAssignedSkills = async () => {
    await qc.invalidateQueries({ queryKey: workspaceKeys.squadSkills(wsId, squad.id) });
  };

  // The runtime skill policy lives on the squad itself.
  const refreshSquad = async () => {
    await qc.invalidateQueries({
      queryKey: squadDetailOptions(wsId, squad.id).queryKey,
      exact: true,
    });
  };

  // Runs one write while its control shows as busy; a failure becomes a
  // toast and leaves the shown state as the server has it.
  const runWrite = async (
    busyKey: string,
    write: () => Promise<unknown>,
    refresh: () => Promise<void>,
    fallbackMessage: string,
  ) => {
    setBusyId(busyKey);
    try {
      await write();
      await refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : fallbackMessage);
    } finally {
      setBusyId(null);
    }
  };

  const handleRemove = (skillId: string) =>
    runWrite(
      skillId,
      () => api.removeSquadSkill(squad.id, skillId),
      refreshAssignedSkills,
      t(($) => $.skills_tab.remove_failed_toast),
    );

  const handleToggle = (skillId: string, enabled: boolean) =>
    runWrite(
      skillId,
      () => api.setSquadSkillEnabled(squad.id, skillId, enabled),
      refreshAssignedSkills,
      t(($) => $.skills_tab.toggle_failed_toast),
    );

  const handleRuntimeToggle = (skill: RuntimeLocalSkillSummary, enabled: boolean) => {
    if (!runtime || !skill.root) return;
    const root = skill.root;
    return runWrite(
      runtimeSkillIdentity(skill),
      () =>
        api.setSquadRuntimeSkillEnabled(squad.id, {
          runtime_id: runtime.id,
          root,
          key: skill.key,
          name: skill.name,
          plugin: skill.plugin,
          enabled,
        }),
      refreshSquad,
      t(($) => $.skills_tab.runtime_toggle_failed_toast),
    );
  };

  const handleRuntimeSectionToggle = (enabled: boolean) =>
    runWrite(
      "runtime:inherit",
      () => api.setSquadRuntimeSkillsInherited(squad.id, enabled),
      refreshSquad,
      t(($) => $.skills_tab.runtime_section_failed_toast),
    );

  const runtimeSkills = runtimeQuery.data?.skills ?? [];
  // Off on a new squad, so it gives no runtime skills until turned on.
  const runtimeSectionOn = squad.inherit_runtime_skills === true;
  const canToggleRuntimeSection =
    canEdit &&
    runtimeQuery.isSuccess &&
    runtimeQuery.data?.supported === true &&
    runtimeSkills.some((skill) => skill.can_disable === true && skill.root);
  const shownSkills = filterBySkillSearch(assignedSkills, query, (skill) => [
    skill.name,
    skill.description,
  ]);
  // A skill is on when its state follows the section switch, unless it is
  // an exception to it on this runtime.
  const isRuntimeSkillOn = (skill: RuntimeLocalSkillSummary) =>
    isRuntimeSkillException(squad.runtime_skill_exceptions, runtime?.id, skill) !==
    runtimeSectionOn;
  // The skills that are on come first, so the squad's own set shows at the top.
  const shownRuntimeSkills = [
    ...filterBySkillSearch(runtimeSkills, query, (skill) => [
      skill.name,
      skill.description,
      skill.source_path,
    ]),
  ].sort((a, b) => Number(isRuntimeSkillOn(b)) - Number(isRuntimeSkillOn(a)));

  return (
    <div className="space-y-8">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t(($) => $.skills_tab.search_placeholder)}
          aria-label={t(($) => $.skills_tab.search_placeholder)}
          className="h-9 pl-8 text-body"
        />
      </div>

      <CapabilitySection
        title={t(($) => $.skills_tab.assigned_title)}
        description={t(($) => $.skills_tab.assigned_hint)}
        action={
          canEdit ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowAdd(true)}
              disabled={workspaceSkills.length === 0}
            >
              <Plus className="h-3.5 w-3.5" />
              {t(($) => $.skills_tab.add_action)}
            </Button>
          ) : null
        }
      >
        {assignedSkills.length === 0 ? (
          <EmptyState
            icon={<SkillIcon className="h-6 w-6" />}
            title={t(($) => $.skills_tab.empty_title)}
          />
        ) : shownSkills.length === 0 ? (
          <RuntimeNotice text={t(($) => $.skills_tab.search_empty, { query: query.trim() })} />
        ) : (
          <ul className="divide-y rounded-lg border bg-surface-raised/40">
            {shownSkills.map((skill) => {
              const enabled = skill.enabled !== false;
              const busy = busyId === skill.id;
              return (
                <li key={skill.id} className="flex items-center gap-3 p-3">
                  <button
                    type="button"
                    onClick={() => setSelected({ kind: "workspace", id: skill.id })}
                    className="flex min-w-0 flex-1 items-center gap-3 rounded-md text-left outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <span
                      className={cn(
                        "flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground",
                        !enabled && "text-faint-foreground",
                      )}
                    >
                      <SkillIcon className="h-4 w-4" />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className={cn("block text-body font-medium", !enabled && "text-muted-foreground")}>
                        <HighlightText text={skill.name} query={query} />
                      </span>
                      <span className="block truncate text-caption text-muted-foreground">
                        {skill.description ? (
                          <HighlightText text={skill.description} query={query} />
                        ) : (
                          t(($) => $.skills_tab.no_description)
                        )}
                      </span>
                    </span>
                  </button>
                  {canEdit && (
                    <>
                      {busy ? (
                        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground motion-reduce:animate-none" />
                      ) : (
                        <Switch
                          checked={enabled}
                          onCheckedChange={(checked) => handleToggle(skill.id, checked)}
                          aria-label={t(($) => $.skills_tab.toggle_aria, { name: skill.name })}
                        />
                      )}
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => handleRemove(skill.id)}
                        disabled={busyId !== null}
                        aria-label={t(($) => $.skills_tab.remove_aria, { name: skill.name })}
                        className="text-muted-foreground hover:text-destructive"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </CapabilitySection>

      <CapabilitySection
        title={t(($) => $.skills_tab.runtime_title)}
        description={t(($) => $.skills_tab.runtime_hint)}
        action={
          runtimeId ? (
            <div className="flex items-center gap-2">
              {canToggleRuntimeSection && runtime && (
                <Switch
                  checked={runtimeSectionOn}
                  disabled={busyId !== null}
                  onCheckedChange={handleRuntimeSectionToggle}
                  aria-label={t(($) => $.skills_tab.runtime_section_aria, {
                    runtime: runtimeDisplayLabel(runtime),
                  })}
                />
              )}
              <Button
                variant="ghost"
                size="sm"
                onClick={() => runtimeQuery.refetch()}
                disabled={runtimeQuery.isFetching}
              >
                <RefreshCw
                  className={cn(
                    "h-3.5 w-3.5",
                    runtimeQuery.isFetching && "animate-spin motion-reduce:animate-none",
                  )}
                />
                {t(($) => $.skills_tab.refresh_action)}
              </Button>
            </div>
          ) : null
        }
      >
        {runtime && (
          <Select
            items={switchableRuntimes.map((r) => ({
              value: r.id,
              label: runtimeDisplayLabel(r),
            }))}
            value={runtime.id}
            onValueChange={(value) => value && setPickedRuntimeId(value)}
          >
            <SelectTrigger
              className="w-full sm:w-72"
              aria-label={t(($) => $.skills_tab.runtime_picker_label)}
            >
              <SelectValue>
                <ProviderLogo provider={runtime.provider} className="h-4 w-4 shrink-0" />
                <span className="truncate">{runtimeDisplayLabel(runtime)}</span>
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              {switchableRuntimes.map((r) => (
                <SelectItem key={r.id} value={r.id}>
                  <ProviderLogo provider={r.provider} className="h-4 w-4 shrink-0" />
                  <span className="truncate">{runtimeDisplayLabel(r)}</span>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        {!runtime ? (
          <RuntimeNotice text={t(($) => $.skills_tab.runtime_none)} />
        ) : !canReadRuntime ? (
          <RuntimeNotice text={t(($) => $.skills_tab.runtime_forbidden)} />
        ) : runtimeQuery.isLoading ? (
          <RuntimeNotice loading text={t(($) => $.skills_tab.runtime_discovering)} />
        ) : runtimeQuery.isError ? (
          <RuntimeNotice
            text={
              runtimeQuery.error instanceof ApiError && runtimeQuery.error.status === 403
                ? t(($) => $.skills_tab.runtime_forbidden)
                : t(($) => $.skills_tab.runtime_failed)
            }
          />
        ) : runtimeQuery.data?.supported !== true ? (
          <RuntimeNotice text={t(($) => $.skills_tab.runtime_unsupported)} />
        ) : runtimeSkills.length === 0 ? (
          <RuntimeNotice text={t(($) => $.skills_tab.runtime_empty)} />
        ) : shownRuntimeSkills.length === 0 ? (
          <RuntimeNotice text={t(($) => $.skills_tab.search_empty, { query: query.trim() })} />
        ) : (
          <ul className="divide-y rounded-lg border bg-surface-raised/40">
            {shownRuntimeSkills.map((skill) => {
              const off = !isRuntimeSkillOn(skill);
              const busyKey = runtimeSkillIdentity(skill);
              return (
                <li key={busyKey} className="flex items-center gap-3 p-3">
                  <button
                    type="button"
                    onClick={() => setSelected({ kind: "runtime", skill })}
                    className="flex min-w-0 flex-1 items-center gap-3 rounded-md text-left outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <span
                      className={cn(
                        "flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground",
                        off && "text-faint-foreground",
                      )}
                    >
                      <Server className="h-4 w-4" />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className={cn("block text-body font-medium", off && "text-muted-foreground")}>
                        <HighlightText text={skill.name} query={query} />
                      </span>
                      <span className="block truncate text-caption text-muted-foreground">
                        <HighlightText text={skill.description || skill.source_path} query={query} />
                      </span>
                    </span>
                  </button>
                  {canEdit &&
                    skill.can_disable === true &&
                    skill.root &&
                    (busyId === busyKey ? (
                      <Loader2 className="h-4 w-4 animate-spin text-muted-foreground motion-reduce:animate-none" />
                    ) : (
                      <Switch
                        checked={!off}
                        onCheckedChange={(checked) => handleRuntimeToggle(skill, checked)}
                        aria-label={t(($) => $.skills_tab.runtime_toggle_aria, { name: skill.name })}
                      />
                    ))}
                </li>
              );
            })}
          </ul>
        )}
      </CapabilitySection>

      <AssignSkillsDialog
        assignedSkillIds={assignedSkills.map((skill) => skill.id)}
        allAssignedMessage={t(($) => $.skills_tab.add_dialog_all_assigned)}
        open={showAdd}
        onOpenChange={setShowAdd}
        onAdd={async (skillIds) => {
          await api.addSquadSkills(squad.id, { skill_ids: skillIds });
          await refreshAssignedSkills();
        }}
      />
      <SkillDetailDialog
        selected={selected}
        onOpenChange={(open) => !open && setSelected(null)}
        workspaceSkill={detailQuery.data}
        loading={selected?.kind === "workspace" && detailQuery.isLoading}
      />
    </div>
  );
}
