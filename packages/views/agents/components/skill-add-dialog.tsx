"use client";

import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type { Agent, SkillSummary } from "@multica/core/types";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  skillListOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../../i18n";
import { SkillPickerList } from "./skill-picker-list";

/** "Attach workspace skills to this agent." */
export function SkillAddDialog({
  agent,
  open,
  onOpenChange,
}: {
  agent: Agent;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const { t } = useT("agents");
  const wsId = useWorkspaceId();
  const qc = useQueryClient();

  return (
    <AssignSkillsDialog
      assignedSkillIds={agent.skills.map((s) => s.id)}
      allAssignedMessage={t(($) => $.tab_body.skills.add_dialog_empty_partial)}
      open={open}
      onOpenChange={onOpenChange}
      onAdd={async (skillIds) => {
        await api.setAgentSkills(agent.id, {
          skill_ids: [...agent.skills.map((s) => s.id), ...skillIds],
        });
        qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
      }}
    />
  );
}

/**
 * "Assign workspace skills." Multi-select with explicit Confirm — earlier
 * iterations attached on a single row click, which meant the user couldn't
 * tick several skills at once and the dialog closed before they could
 * review their choice.
 *
 * Already-assigned skills are filtered out of the list entirely (vs.
 * showing them disabled). When there are no remaining workspace skills
 * to assign, the empty-state copy explains why, and the Confirm button
 * is naturally disabled because nothing can be selected.
 *
 * `onAdd` receives only the newly picked skills; the caller assigns them
 * to its owner (an agent or a squad).
 */
export function AssignSkillsDialog({
  assignedSkillIds,
  allAssignedMessage,
  onAdd,
  open,
  onOpenChange,
}: {
  assignedSkillIds: readonly string[];
  /** Shown when every workspace skill is already assigned to the owner. */
  allAssignedMessage: string;
  onAdd: (skillIds: string[]) => Promise<void>;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const { t } = useT("agents");
  const wsId = useWorkspaceId();
  const { data: workspaceSkills = [], isLoading } = useQuery(skillListOptions(wsId));
  const [saving, setSaving] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  // Hide assigned skills outright — the dialog is for adding new ones.
  // The Skills tab list above shows what the owner already has.
  const availableSkills = useMemo(() => {
    const assigned = new Set(assignedSkillIds);
    return workspaceSkills.filter((s) => !assigned.has(s.id));
  }, [workspaceSkills, assignedSkillIds]);

  const handleOpenChange = (v: boolean) => {
    if (!v) setSelectedIds(new Set());
    onOpenChange(v);
  };

  const handleToggle = (skill: SkillSummary) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(skill.id)) next.delete(skill.id);
      else next.add(skill.id);
      return next;
    });
  };

  const handleConfirm = async () => {
    if (selectedIds.size === 0) return;
    setSaving(true);
    try {
      await onAdd([...selectedIds]);
      handleOpenChange(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.tab_body.skills.add_failed_toast));
    } finally {
      setSaving(false);
    }
  };

  const count = selectedIds.size;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="text-body">
            {t(($) => $.tab_body.skills.add_dialog_title)}
          </DialogTitle>
        </DialogHeader>

        <SkillPickerList
          skills={availableSkills}
          selectedIds={selectedIds}
          onToggle={handleToggle}
          loading={isLoading}
          emptyMessage={
            workspaceSkills.length === 0
              ? t(($) => $.tab_body.skills.add_dialog_empty)
              : allAssignedMessage
          }
          noMatchMessage={t(($) => $.tab_body.skills.add_dialog_no_match)}
        />

        <DialogFooter>
          <Button variant="ghost" onClick={() => handleOpenChange(false)}>
            {t(($) => $.tab_body.skills.add_dialog_cancel)}
          </Button>
          <Button
            onClick={handleConfirm}
            disabled={count === 0 || saving}
          >
            {saving
              ? t(($) => $.tab_body.skills.add_dialog_saving)
              : count > 0
                ? t(($) => $.tab_body.skills.add_dialog_confirm, { count })
                : t(($) => $.tab_body.skills.add_dialog_confirm_default)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
