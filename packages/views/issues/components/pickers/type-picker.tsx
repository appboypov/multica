"use client";

import { useState } from "react";
import { ISSUE_TYPES, type IssueType } from "@multica/core/types";
import { Tooltip, TooltipContent, TooltipTrigger } from "@multica/ui/components/ui/tooltip";
import { TypeIcon } from "../type-icon";
import { PropertyPicker, PickerItem, PICKER_TRIGGER_CLASS } from "./property-picker";
import { useT } from "../../../i18n";

interface TypePickerProps {
  /** The issue's type; null for an issue from before types. */
  type: IssueType | null;
  /** Sets the type once. */
  onPick: (type: IssueType) => void;
  align?: "start" | "center" | "end";
}

/**
 * Type on the issue page. An issue without a type offers the eleven once; a
 * set type shows itself and offers no other (the `issue-types` spec delta).
 */
export function TypePicker({ type, onPick, align }: TypePickerProps) {
  const [open, setOpen] = useState(false);
  const { t } = useT("issues");
  const label = type ? t(($) => $.types[type].name) : t(($) => $.types.none);
  const trigger = (
    <>
      <TypeIcon type={type} />
      <span className="truncate">{label}</span>
    </>
  );

  if (type) {
    return (
      <Tooltip>
        <TooltipTrigger render={<span className={PICKER_TRIGGER_CLASS}>{trigger}</span>} />
        <TooltipContent side="top">{t(($) => $.pickers.type.fixed)}</TooltipContent>
      </Tooltip>
    );
  }

  return (
    <PropertyPicker open={open} onOpenChange={setOpen} width="w-48" align={align} trigger={trigger}>
      {ISSUE_TYPES.map((option) => (
        <PickerItem
          key={option}
          selected={false}
          onClick={() => {
            onPick(option);
            setOpen(false);
          }}
        >
          <TypeIcon type={option} />
          <span className="truncate">{t(($) => $.types[option].name)}</span>
        </PickerItem>
      ))}
    </PropertyPicker>
  );
}
