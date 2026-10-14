"use client";

import type { Issue, UpdateIssueRequest } from "@multica/core/types";
import { PillButton } from "../../common/pill-button";
import { ProjectPicker } from "../../projects/components/project-picker";
import {
  DueDatePicker,
  LabelPicker,
  PriorityPicker,
  StartDatePicker,
} from "./pickers";

/**
 * The issue's other core properties as one wrapping row of pills, each
 * opening the same picker the detail sidebar uses. Used where there is no room
 * for the sidebar — the side peek, under its summary of status, worklane,
 * release and assignee — so triage stays one click per field.
 *
 * Start date is shown only once set: it is the least-used field, and an empty
 * pill for it would push the rest onto a second line. Custom properties stay
 * on the full page.
 */
export function IssuePropertyPills({
  issue,
  onUpdate,
}: {
  issue: Issue;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
}) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <PriorityPicker
        priority={issue.priority}
        onUpdate={onUpdate}
        triggerRender={<PillButton />}
        align="start"
      />
      <ProjectPicker
        projectId={issue.project_id}
        onUpdate={onUpdate}
        triggerRender={<PillButton />}
      />
      {issue.start_date && (
        <StartDatePicker
          startDate={issue.start_date}
          onUpdate={onUpdate}
          triggerRender={<PillButton />}
          align="start"
        />
      )}
      <DueDatePicker
        dueDate={issue.due_date}
        onUpdate={onUpdate}
        triggerRender={<PillButton />}
        align="start"
      />
      <LabelPicker issueId={issue.id} triggerRender={<PillButton />} align="start" />
    </div>
  );
}
