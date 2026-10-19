import {
  Bookmark,
  Bug,
  CircleDashed,
  ClipboardCheck,
  Flag,
  FlaskConical,
  GitCompareArrows,
  RefreshCw,
  Route,
  Sparkles,
  Sunrise,
  Wrench,
  type LucideIcon,
} from "lucide-react";
import type { IssueType } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";

/** The eleven type glyphs of the library, each drawn in its `--type-*` colour. */
export const TYPE_ICON: Record<IssueType, LucideIcon> = {
  epic: Flag,
  journey: Route,
  story: Bookmark,
  bug: Bug,
  enhancement: Sparkles,
  refactor: GitCompareArrows,
  chore: Wrench,
  spike: FlaskConical,
  standup: Sunrise,
  workcycle: RefreshCw,
  checkout: ClipboardCheck,
};

const TYPE_COLOR: Record<IssueType, string> = {
  epic: "text-type-epic",
  journey: "text-type-journey",
  story: "text-type-story",
  bug: "text-type-bug",
  enhancement: "text-type-enhancement",
  refactor: "text-type-refactor",
  chore: "text-type-chore",
  spike: "text-type-spike",
  standup: "text-type-standup",
  workcycle: "text-type-workcycle",
  checkout: "text-type-checkout",
};

/**
 * An issue's type icon, with its type name as accessible name. An issue
 * without a type shows a dashed circle named "No type".
 */
export function TypeIcon({
  type,
  className,
}: {
  type: IssueType | null | undefined;
  className?: string;
}) {
  const { t } = useT("issues");
  if (!type) {
    return (
      <CircleDashed
        role="img"
        aria-label={t(($) => $.types.none)}
        className={cn("size-3.5 shrink-0 text-faint-foreground", className)}
      />
    );
  }
  const Icon = TYPE_ICON[type];
  return (
    <Icon
      role="img"
      aria-label={t(($) => $.types[type].name)}
      className={cn("size-3.5 shrink-0", TYPE_COLOR[type], className)}
    />
  );
}
