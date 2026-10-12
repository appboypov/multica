"use client";

import { useT } from "../../i18n";

// Renders a single inline line so it can sit under any textarea / input
// without disturbing surrounding spacing.
//
// Hard (default): `max` is a cap the input enforces; warn at 90 % of it and
// show an error past it.
// Soft: `max` is only a recommended length; text past it is allowed and
// saved, so there is no near tone and the overflow reads as a warning.
export function CharCounter({
  length,
  max,
  soft = false,
}: {
  length: number;
  max: number;
  soft?: boolean;
}) {
  const { t } = useT("agents");
  const over = length > max;
  const near = !soft && !over && length >= Math.floor(max * 0.9);
  const tone = over
    ? soft
      ? "text-warning"
      : "text-destructive"
    : near
      ? "text-warning"
      : "text-muted-foreground";
  return (
    <div className={`text-right text-caption tabular-nums ${tone}`}>
      {length} / {max}
      {over &&
        (soft
          ? t(($) => $.char_counter.over_threshold, {
              count: length - max,
              max,
            })
          : t(($) => $.char_counter.over_limit, { count: length - max }))}
    </div>
  );
}
