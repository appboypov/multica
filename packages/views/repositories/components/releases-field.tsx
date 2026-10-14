"use client";

import { useEffect, useMemo } from "react";
import type { VersionPolicy } from "@multica/core/types";
import { Label } from "@multica/ui/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { useT } from "../../i18n";

const VERSION_FILES: readonly VersionPolicy[] = [
  "pubspec.yaml",
  "Cargo.toml",
  "pyproject.toml",
  "package.json",
];

/**
 * Releases: one option per version file the check found on the default
 * branch, in policy order, then Tags only and This repository does not
 * release. The first option is picked when the value is not one of them, so
 * a repository without a version file starts on Tags only. `keep` stays
 * offered even when the check did not find it, so Edit shows the saved policy.
 */
export function ReleasesField({
  versionFiles,
  lastTag,
  value,
  onChange,
  keep,
  disabled,
}: {
  /** The version files of the check; null before a repository is picked and checked. */
  versionFiles: readonly string[] | null;
  /** The highest `v{version}` tag without `v`, "" for none, null while not known. */
  lastTag: string | null;
  value: VersionPolicy | null;
  onChange: (value: VersionPolicy) => void;
  keep?: VersionPolicy;
  disabled?: boolean;
}) {
  const { t } = useT("repositories");

  const options = useMemo(() => {
    if (versionFiles === null) return keep ? [keep] : [];
    const files = VERSION_FILES.filter(
      (file) => versionFiles.includes(file) || file === keep,
    );
    return [...files, "tags", "none"] as VersionPolicy[];
  }, [versionFiles, keep]);

  useEffect(() => {
    const [first] = options;
    if (first && (value === null || !options.includes(value))) onChange(first);
  }, [options, value, onChange]);

  const label = (policy: VersionPolicy) => {
    if (policy === "none") return t(($) => $.releases.none);
    if (policy === "tags") return t(($) => $.releases.tags);
    return t(($) => $.releases.version_file, { file: policy });
  };

  const items = options.map((policy) => ({ value: policy, label: label(policy) }));

  return (
    <div className="flex flex-col gap-2">
      <Label>{t(($) => $.releases.label)}</Label>
      <Select
        items={items}
        value={value}
        onValueChange={(next) => {
          if (next) onChange(next as VersionPolicy);
        }}
        disabled={disabled || options.length === 0}
      >
        <SelectTrigger aria-label={t(($) => $.releases.label)} className="w-full">
          <SelectValue placeholder={t(($) => $.releases.pick_repository_first)}>
            {() => (value ? label(value) : t(($) => $.releases.pick_repository_first))}
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {value === "none" ? (
        <p className="text-caption text-muted-foreground">{t(($) => $.releases.none_note)}</p>
      ) : value !== null && lastTag !== null ? (
        <p className="text-caption text-muted-foreground">
          {lastTag === ""
            ? t(($) => $.releases.no_tags)
            : t(($) => $.releases.last_tag, { tag: `v${lastTag}` })}
        </p>
      ) : null}
    </div>
  );
}
