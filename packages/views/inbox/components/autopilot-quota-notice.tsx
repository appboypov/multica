"use client";

import type { InboxItem } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";

function formatResetAt(value: unknown): string {
  if (typeof value !== "string" || !value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

export function AutopilotQuotaNotice({
  item,
  onOpenRecovery,
}: {
  /** A quota notice, or a Quota limit reached to-do: both carry the notice's details. */
  item: Pick<InboxItem, "body"> & { details: Readonly<Record<string, unknown>> | null };
  onOpenRecovery: () => void;
}) {
  const { t } = useT("inbox");
  const details = item.details ?? {};
  const resetAt = formatResetAt(details.reset_at);
  const limit = typeof details.limit === "string" || typeof details.limit === "number" ? String(details.limit) : "";

  let body = item.body;
  if (limit && resetAt) {
    body = t(($) => $.detail.autopilot_quota_exceeded_body, {
      limit,
      resetAt,
    });
  }

  return (
    <>
      {body && (
        <div className="mt-4 whitespace-pre-wrap text-body leading-relaxed text-foreground">
          {body}
        </div>
      )}
      <Button className="mt-4" size="sm" onClick={onOpenRecovery}>
        {t(($) => $.detail.autopilot_quota_upgrade)}
      </Button>
    </>
  );
}
