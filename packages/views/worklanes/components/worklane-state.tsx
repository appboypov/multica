"use client";

import type { WorklaneState } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";

/** A worklane's state as a dot and a word: Free, or Busy while an issue of it is worked on. */
export function WorklaneStateLabel({
  state,
  className,
}: {
  state: WorklaneState;
  className?: string;
}) {
  const { t } = useT("worklanes");
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1.5 text-caption",
        state === "busy" ? "text-foreground" : "text-muted-foreground",
        className,
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "size-1.5 rounded-full",
          state === "busy" ? "bg-warning" : "bg-success",
        )}
      />
      {t(($) => $.state[state])}
    </span>
  );
}
