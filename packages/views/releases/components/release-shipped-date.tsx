"use client";

import { useLocale } from "../../i18n";

export function ReleaseShippedDate({ shippedAt }: { shippedAt: string | null }) {
  const locale = useLocale();
  if (!shippedAt) return null;
  return (
    <time dateTime={shippedAt} className="shrink-0 text-caption text-muted-foreground">
      {new Date(shippedAt).toLocaleDateString(locale, { dateStyle: "medium" })}
    </time>
  );
}
