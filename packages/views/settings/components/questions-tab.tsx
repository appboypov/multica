"use client";

import { useId, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { useWorkspaceId } from "@multica/core/hooks";
import { useAuthStore } from "@multica/core/auth";
import { memberListOptions } from "@multica/core/workspace/queries";
import { agentQuestionsOptions, useUpdateAgentQuestionsDefault } from "@multica/core/questions";
import { Switch } from "@multica/ui/components/ui/switch";
import { useT } from "../../i18n";
import { SettingsCard, SettingsRow, SettingsSection, SettingsTab } from "./settings-layout";

/**
 * The workspace Questions default: whether agents may ask questions on an
 * issue on Workspace default. Each issue can change it in its properties.
 */
export function QuestionsTab() {
  const { t } = useT("settings");
  const wsId = useWorkspaceId();
  const id = useId();
  const { data: settings, isError } = useQuery(agentQuestionsOptions(wsId));
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const currentUser = useAuthStore((s) => s.user);
  const canManage = useMemo(() => {
    const role = members.find((m) => m.user_id === currentUser?.id)?.role;
    return role === "owner" || role === "admin";
  }, [members, currentUser]);
  const update = useUpdateAgentQuestionsDefault(wsId);

  return (
    <SettingsTab title={t(($) => $.page.tabs.questions)} description={t(($) => $.questions.description)}>
      {isError && (
        <p role="alert" className="text-caption text-destructive">
          {t(($) => $.questions.load_error)}
        </p>
      )}
      {settings && (
        <SettingsSection anchor="agent-questions" title={t(($) => $.questions.card_title)}>
          <SettingsCard>
            <SettingsRow
              label={<label htmlFor={`${id}-enabled`}>{t(($) => $.questions.enabled)}</label>}
              description={
                settings.own_setting_count > 0
                  ? t(($) => $.questions.customized, { count: settings.own_setting_count })
                  : undefined
              }
            >
              <Switch
                id={`${id}-enabled`}
                checked={settings.enabled}
                disabled={!canManage || update.isPending}
                aria-busy={update.isPending || undefined}
                onCheckedChange={(enabled) =>
                  update.mutate(enabled, { onError: () => toast.error(t(($) => $.questions.save_error)) })
                }
              />
            </SettingsRow>
            <div className="space-y-2 px-4 py-3.5">
              <p className="text-body font-medium">{t(($) => $.questions.brief_title)}</p>
              <pre className="max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted/60 px-3 py-2 font-mono text-caption leading-5 text-muted-foreground">
                {settings.brief_section}
              </pre>
              <p className="text-caption leading-5 text-muted-foreground">{t(($) => $.questions.off_line)}</p>
            </div>
          </SettingsCard>
        </SettingsSection>
      )}
      {settings && !canManage && (
        <p className="text-caption text-muted-foreground">{t(($) => $.questions.admin_only)}</p>
      )}
    </SettingsTab>
  );
}
