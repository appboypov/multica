"use client";

import { useEffect, useId, useState } from "react";
import type { RepositoryInstallCommand } from "@multica/core/types";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { Spinner } from "@multica/ui/components/ui/spinner";
import { useT } from "../../i18n";

const NONE = "__none__";
const TYPED = "__typed__";

/**
 * Install: the command that installs a new worklane's dependencies. It offers
 * the commands the checkout check found on the default branch, in table
 * order, then None, then a typed command. While the value is not known yet it
 * fills in the first found command, or None when the check found none. A
 * value that is not one of the found commands shows as typed, so Edit shows
 * the saved command before its check answers.
 */
export function InstallField({
  found,
  value,
  onChange,
  finding,
  disabled,
}: {
  /** The commands of the check; null while there is no check result. */
  found: readonly RepositoryInstallCommand[] | null;
  /** "" means None; null means not known yet. */
  value: string | null;
  onChange: (value: string) => void;
  /** A check runs or will run for this repository: says "Finding the install command" while unknown. */
  finding: boolean;
  disabled?: boolean;
}) {
  const { t } = useT("repositories");
  const inputId = useId();
  const [typing, setTyping] = useState(false);

  useEffect(() => {
    if (value === null && found !== null) onChange(found[0]?.command ?? "");
  }, [value, found, onChange]);

  const commands = found ?? [];
  const isFound = value !== null && commands.some((c) => c.command === value);
  const mode =
    value === null
      ? null
      : typing || (value !== "" && !isFound)
        ? TYPED
        : value === ""
          ? NONE
          : value;

  const items = [
    ...commands.map((c) => ({ value: c.command, label: c.command, file: c.file })),
    { value: NONE, label: t(($) => $.install.none), file: "" },
    { value: TYPED, label: t(($) => $.install.typed), file: "" },
  ];
  const label = (selected: string | null) =>
    selected === null
      ? finding
        ? t(($) => $.install.finding)
        : t(($) => $.install.pick_repository_first)
      : (items.find((item) => item.value === selected)?.label ?? selected);

  return (
    <div className="flex flex-col gap-2">
      <Label>{t(($) => $.install.label)}</Label>
      <Select
        items={items}
        value={mode}
        onValueChange={(next) => {
          if (!next) return;
          if (next === TYPED) {
            setTyping(true);
            return;
          }
          setTyping(false);
          onChange(next === NONE ? "" : next);
        }}
        disabled={disabled || value === null}
      >
        <SelectTrigger aria-label={t(($) => $.install.label)} className="w-full">
          <SelectValue placeholder={label(null)}>
            {() => (
              <span className="flex min-w-0 items-center gap-2">
                {mode === null && finding ? <Spinner className="size-3.5" /> : null}
                <span className={mode !== null && mode !== NONE && mode !== TYPED ? "font-mono" : undefined}>
                  {label(mode)}
                </span>
              </span>
            )}
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              <span className={item.file ? "font-mono" : undefined}>{item.label}</span>
              {item.file ? (
                <span className="ml-auto pl-3 text-caption text-muted-foreground">
                  {t(($) => $.install.from_file, { file: item.file })}
                </span>
              ) : null}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {mode === TYPED ? (
        <Input
          id={inputId}
          aria-label={t(($) => $.install.typed)}
          className="font-mono"
          placeholder={t(($) => $.install.typed_placeholder)}
          value={value ?? ""}
          onChange={(e) => onChange(e.target.value)}
          disabled={disabled}
        />
      ) : null}
      {mode === NONE ? (
        <p className="text-caption text-muted-foreground">{t(($) => $.install.none_note)}</p>
      ) : null}
    </div>
  );
}
