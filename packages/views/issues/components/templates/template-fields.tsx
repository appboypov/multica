"use client";

import type { IssueTemplate } from "@multica/core/types";
import type { TemplateFields as TemplateFieldValues } from "@multica/core/issues/templates";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../../i18n";

interface TemplateFieldsProps {
  template: IssueTemplate;
  fields: TemplateFieldValues;
  onChange: (key: string, value: string) => void;
}

/**
 * The picked template's fields in section order, each marked Required or
 * Optional. Labels and placeholders are the template's own text: they become
 * the issue's Markdown, so they stay as written in every language.
 */
export function TemplateFields({ template, fields, onChange }: TemplateFieldsProps) {
  const { t } = useT("issues");
  return (
    <div className="flex flex-col gap-4">
      {template.sections.map((section) => {
        const field = section.field;
        if (!field) return null;
        const id = `template-field-${template.type}-${field.key}`;
        return (
          <div key={`${template.type}-${field.key}`} className="flex flex-col gap-1.5">
            <label htmlFor={id} className="flex items-baseline gap-2">
              <span className="text-body font-medium">{field.label}</span>
              <span className="text-caption text-muted-foreground">
                {field.required ? t(($) => $.templates.required) : t(($) => $.templates.optional)}
              </span>
            </label>
            <Textarea
              id={id}
              value={fields[field.key] ?? ""}
              onChange={(e) => onChange(field.key, e.target.value)}
              placeholder={field.placeholder}
              rows={field.rows}
              aria-required={field.required}
              className="resize-none"
            />
          </div>
        );
      })}
    </div>
  );
}
