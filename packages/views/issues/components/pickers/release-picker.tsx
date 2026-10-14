"use client";

import { useState } from "react";
import { CircleDashed, Milestone } from "lucide-react";
import { unshippedReleases } from "@multica/core/issues/placement";
import { formatVersion } from "@multica/core/releases";
import { PropertyPicker, PickerItem } from "./property-picker";
import { usePlacementData } from "./use-placement-data";
import { useT } from "../../../i18n";

interface ReleasePickerProps {
  /** The worklane whose repository's line the releases come from. */
  worklaneId: string | null;
  releaseId: string | null;
  onPick: (releaseId: string | null) => void;
  align?: "start" | "center" | "end";
  trigger?: React.ReactNode;
  /** The element the trigger renders as, such as New issue's pill. */
  triggerRender?: React.ReactElement;
}

/**
 * Release in New issue and on the issue page: the unshipped releases of the
 * worklane's repository by version and title, then "No release". Without a
 * worklane it says "Pick a worklane first".
 */
export function ReleasePicker({
  worklaneId,
  releaseId,
  onPick,
  align,
  trigger,
  triggerRender,
}: ReleasePickerProps) {
  const [open, setOpen] = useState(false);
  const { t } = useT("issues");
  const data = usePlacementData();

  const worklane = worklaneId ? data.worklanes.find((w) => w.id === worklaneId) : undefined;
  const releases = worklane ? unshippedReleases(data.releases, worklane.repository_id) : [];
  const picked = releaseId ? data.releases.find((r) => r.id === releaseId) : undefined;

  const pick = (id: string | null) => {
    onPick(id);
    setOpen(false);
  };

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-72"
      align={align}
      triggerRender={triggerRender}
      trigger={
        trigger ?? (
          <>
            {picked ? (
              <Milestone className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            ) : (
              <CircleDashed className="h-3.5 w-3.5 shrink-0 text-faint-foreground" />
            )}
            {picked ? (
              <>
                <span className="shrink-0 font-mono">{formatVersion(picked.version)}</span>
                <span className="truncate">{picked.title}</span>
              </>
            ) : (
              <span className="truncate">{t(($) => $.pickers.release.none)}</span>
            )}
          </>
        )
      }
    >
      {releases.map((release) => (
        <PickerItem
          key={release.id}
          selected={release.id === releaseId}
          onClick={() => pick(release.id)}
        >
          <span className="shrink-0 font-mono">{formatVersion(release.version)}</span>
          <span className="truncate text-muted-foreground">{release.title}</span>
        </PickerItem>
      ))}
      {releases.length === 0 && (
        <div className="px-2 py-1.5 text-caption text-muted-foreground">
          {worklane
            ? t(($) => $.pickers.release.no_releases)
            : t(($) => $.pickers.release.pick_worklane_first)}
        </div>
      )}
      <PickerItem selected={releaseId === null} emptyValue onClick={() => pick(null)}>
        <span className="truncate text-muted-foreground">{t(($) => $.pickers.release.none)}</span>
      </PickerItem>
    </PropertyPicker>
  );
}
