"use client";

import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { ApiError, errorCode } from "@multica/core/api";
import { useRetrySourceContextQuickCreate } from "@multica/core/inbox/mutations";
import { useIssueDraftStore } from "@multica/core/issues/stores/draft-store";
import { issueTemplatesOptions } from "@multica/core/issues/templates";
import { useModalStore } from "@multica/core/modals";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { useIssueLimitUpgradePrompt } from "../../modals/use-issue-limit-upgrade-prompt";

function detailString(details: Readonly<Record<string, unknown>> | null, key: string): string | undefined {
  const value = details?.[key];
  return typeof value === "string" && value ? value : undefined;
}

/**
 * The ways back from a quick create that made no issue: retry with the
 * original context, when the run kept one, and the advanced form seeded with
 * the original prompt. Shared by the Quick create failed to-do and the
 * quick-create notices still listed in Updates.
 */
export function QuickCreateRecovery({
  details,
  retryable,
}: {
  details: Readonly<Record<string, unknown>> | null;
  /** Only a failed quick create can be retried; an unconfirmed one may have made its issue. */
  retryable: boolean;
}) {
  const { t } = useT("inbox");
  const { data: templates = [] } = useQuery(issueTemplatesOptions());
  const showIssueLimitUpgradePrompt = useIssueLimitUpgradePrompt();
  const retrySourceContextMutation = useRetrySourceContextQuickCreate();
  const taskId = detailString(details, "task_id");
  const canRetry = retryable && !!detailString(details, "source_context_id");

  return (
    <>
      {canRetry && taskId && (
        <Button
          size="sm"
          data-testid="retry-source-context"
          disabled={retrySourceContextMutation.isPending}
          onClick={async () => {
            try {
              await retrySourceContextMutation.mutateAsync(taskId);
              toast.success(t(($) => $.toasts.source_context_retry_started));
            } catch (error) {
              if (error instanceof ApiError && errorCode(error) === "issue_limit_reached") {
                showIssueLimitUpgradePrompt();
                return;
              }
              toast.error(
                error instanceof ApiError && errorCode(error) === "source_context_retry_unavailable"
                  ? t(($) => $.errors.source_context_retry_unavailable)
                  : t(($) => $.errors.source_context_retry_failed),
              );
            }
          }}
        >
          {t(($) => $.detail.retry_with_context)}
        </Button>
      )}
      <Button
        size="sm"
        onClick={() => {
          // Seed the legacy advanced form with the original prompt so the
          // user can recover their input in the full editor instead of
          // retyping. The agent picker hint becomes the assignee
          // candidate (still editable).
          const prompt = detailString(details, "original_prompt") ?? "";
          const agentId = detailString(details, "agent_id");
          // The prompt fills the first field of the draft's template, or
          // the free description with No template.
          const { manual } = useIssueDraftStore.getState().draft;
          const firstField = templates
            .find((template) => template.type === manual.type)
            ?.sections.find((section) => section.field)?.field;
          useIssueDraftStore.getState().setManual({
            ...(manual.type === null
              ? { description: prompt }
              : firstField && prompt
                ? { fields: { ...manual.fields, [firstField.key]: prompt } }
                : {}),
            ...(agentId ? { assigneeType: "agent" as const, assigneeId: agentId } : {}),
          });
          useModalStore.getState().open("create-issue");
        }}
      >
        {t(($) => $.detail.edit_advanced)}
      </Button>
    </>
  );
}
