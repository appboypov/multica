"use client";

import { MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { useModalStore } from "@multica/core/modals";
import type { Worklane } from "@multica/core/types";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";

/**
 * Rename and Remove of a worklane, on its page's header and on its rows.
 * `main` has no menu: it cannot be renamed or removed.
 */
export function WorklaneMenu({
  worklane,
  className,
}: {
  worklane: Worklane;
  className?: string;
}) {
  const { t } = useT("worklanes");
  const openModal = useModalStore((s) => s.open);
  if (worklane.is_main) return null;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <button
            type="button"
            aria-label={t(($) => $.menu.label)}
            className={cn(
              "flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground data-popup-open:bg-accent data-popup-open:text-accent-foreground",
              className,
            )}
          >
            <MoreHorizontal className="size-4" />
          </button>
        }
      />
      <DropdownMenuContent align="end" className="w-40">
        <DropdownMenuItem onClick={() => openModal("rename-worklane", { worklaneId: worklane.id })}>
          <Pencil className="size-3.5" />
          {t(($) => $.menu.rename)}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          variant="destructive"
          onClick={() => openModal("remove-worklane", { worklaneId: worklane.id })}
        >
          <Trash2 className="size-3.5" />
          {t(($) => $.menu.remove)}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
