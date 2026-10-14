"use client";

import { Pin } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@multica/ui/components/ui/tooltip";
import { useT } from "../../i18n";

/** Marks a release whose version was set by hand: it keeps that number when it moves. */
export function SetByHand() {
  const { t } = useT("releases");
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span className="inline-flex shrink-0 items-center gap-1 text-caption text-muted-foreground">
            <Pin className="size-3" aria-hidden="true" />
            {t(($) => $.column.set_by_hand)}
          </span>
        }
      />
      <TooltipContent>{t(($) => $.column.set_by_hand_tooltip)}</TooltipContent>
    </Tooltip>
  );
}
