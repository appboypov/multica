"use client";

import type { IssueTemplate } from "@multica/core/types";
import type { TemplateFields as TemplateFieldValues } from "@multica/core/issues/templates";
import { Textarea } from "@multica/ui/components/ui/textarea";

interface TemplateFieldsProps {
  template: IssueTemplate;
  fields: TemplateFieldValues;
  onChange: (key: string, value: string) => void;
}

/**
 * The picked template's fields in section order. Labels and placeholders are
 * the template's own text: they become the issue's Markdown, so they stay as
 * written in every language.
 */
export function TemplateFields({ template, fields, onChange }: TemplateFieldsProps) {
  return (
    <div className="flex flex-col gap-4">
      {template.sections.map((section) => {
        const field = section.field;
        if (!field) return null;
        const id = `template-field-${template.type}-${field.key}`;
        return (
          <div key={`${template.type}-${field.key}`} className="flex flex-col gap-1.5">
            <label htmlFor={id} className="text-body font-medium">
              {field.label}
            </label>
            <Textarea
              id={id}
              value={fields[field.key] ?? ""}
              onChange={(e) => onChange(field.key, e.target.value)}
              placeholder={field.placeholder}
              rows={field.rows}
              className="resize-none"
            />
          </div>
        );
      })}
    </div>
  );
}
