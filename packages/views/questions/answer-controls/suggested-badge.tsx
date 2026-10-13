"use client";

import { Badge } from "@multica/ui/components/ui/badge";
import { useT } from "../../i18n";

/** The library's `Badge/Brand` "Suggested" beside the asker's suggestion. */
export function SuggestedBadge() {
  const { t } = useT("questions");
  return (
    <Badge variant="outline" className="rounded-sm border-brand text-brand">
      {t(($) => $.controls.suggested)}
    </Badge>
  );
}
