"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { agentQuestionsOptions } from "@multica/core/questions";
import type { UpdateIssueRequest } from "@multica/core/types";
import { PropertyPicker, PickerItem } from "./property-picker";
import { useT } from "../../../i18n";

interface AgentQuestionsPickerProps {
  /** The issue's own setting: true On, false Off, null Workspace default. */
  value: boolean | null;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
  align?: "start" | "center" | "end";
}

/** Whether agents may ask questions on one issue: Workspace default, On or Off. */
export function AgentQuestionsPicker({ value, onUpdate, align }: AgentQuestionsPickerProps) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const { data: settings } = useQuery(agentQuestionsOptions(wsId));
  const [open, setOpen] = useState(false);
  const onOff = (enabled: boolean) =>
    enabled ? t(($) => $.detail.agent_questions_on) : t(($) => $.detail.agent_questions_off);
  const workspaceDefault = t(($) => $.detail.agent_questions_workspace_default, {
    value: onOff(settings?.enabled ?? true),
  });
  const items: { value: boolean | null; label: string }[] = [
    { value: null, label: workspaceDefault },
    { value: true, label: onOff(true) },
    { value: false, label: onOff(false) },
  ];
  const selected = items.find((item) => item.value === value) ?? items[0]!;

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-52"
      align={align}
      trigger={<span className="truncate">{selected.label}</span>}
    >
      {items.map((item) => (
        <PickerItem
          key={String(item.value)}
          selected={item.value === value}
          onClick={() => {
            onUpdate({ agent_questions: item.value });
            setOpen(false);
          }}
        >
          {item.label}
        </PickerItem>
      ))}
    </PropertyPicker>
  );
}
