"use client";

import { Check, Circle, Loader2, Minus, X } from "lucide-react";
import type {
  WorklaneRequest,
  WorklaneRequestStep,
  WorklaneRequestStepKey,
} from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";

/** The steps of a create request in the order the runtime runs them. */
const CREATE_STEPS: readonly WorklaneRequestStepKey[] = ["fetch", "add_worktree", "install", "open"];

/**
 * The steps of a create request before the runtime reported any: each
 * pending, so the list shows what will run from the first poll on.
 */
function stepsOf(request: WorklaneRequest): WorklaneRequestStep[] {
  if (request.steps.length > 0) return request.steps;
  return CREATE_STEPS.map((key) => ({ key, state: "pending", detail: "", output: "", duration_ms: 0 }));
}

/**
 * A create request's steps, each pending, running, done, skipped or failed,
 * with what it works on: the base it fetches, the worktree path, the install
 * command. A finished fetch shows how long it took; a failed step shows the
 * runtime's error and output. An unknown step or state from a newer server
 * reads as failed with the server's detail.
 */
export function WorklaneRequestSteps({ request }: { request: WorklaneRequest }) {
  const { t } = useT("worklanes");

  const subject = (step: WorklaneRequestStep): string => {
    switch (step.key) {
      case "fetch":
        return step.state === "done" && step.duration_ms > 0
          ? t(($) => $.steps.took, { seconds: (step.duration_ms / 1000).toFixed(1) })
          : "";
      case "add_worktree":
        return request.path;
      case "install":
        return step.state === "skipped" || request.install_command === ""
          ? t(($) => $.steps.skipped)
          : request.install_command;
      default:
        return "";
    }
  };

  const label = (key: WorklaneRequestStepKey): string => {
    switch (key) {
      case "fetch":
        return t(($) => $.steps.fetch, { base: request.base });
      case "add_worktree":
      case "install":
      case "open":
        return t(($) => $.steps[key]);
      default:
        return t(($) => $.steps.unknown);
    }
  };

  return (
    <ol className="space-y-2">
      {stepsOf(request).map((step, index) => {
        const failed = step.state === "failed" || step.state === "unknown" || step.key === "unknown";
        const state = failed ? "failed" : step.state;
        const secondary = subject(step);
        // A failure keeps the runtime's own words; otherwise the detail is
        // what the step found, such as a worktree that was already there.
        const note = failed ? step.detail : step.key === "fetch" ? "" : step.detail;
        return (
          <li key={`${step.key}-${index}`} className="flex items-start gap-2.5">
            <StepIcon state={state} label={t(($) => $.steps.state[state])} />
            <div className="min-w-0 flex-1 space-y-0.5">
              <div className="flex flex-wrap items-baseline gap-x-2">
                <span className={cn("text-body", state === "pending" ? "text-muted-foreground" : "text-foreground")}>
                  {label(step.key)}
                </span>
                {secondary ? (
                  <span className="truncate font-mono text-caption text-muted-foreground">{secondary}</span>
                ) : null}
              </div>
              {note ? (
                <p className={cn("text-caption", failed ? "text-destructive" : "text-muted-foreground")}>{note}</p>
              ) : null}
              {failed && step.output ? (
                <pre className="max-h-40 overflow-auto rounded-md border bg-muted/40 p-2 font-mono text-caption whitespace-pre-wrap text-foreground">
                  {step.output}
                </pre>
              ) : null}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

function StepIcon({
  state,
  label,
}: {
  state: "pending" | "running" | "done" | "skipped" | "failed";
  label: string;
}) {
  const className = "mt-0.5 size-4 shrink-0";
  switch (state) {
    case "running":
      return <Loader2 aria-label={label} className={cn(className, "animate-spin text-foreground")} />;
    case "done":
      return <Check aria-label={label} className={cn(className, "text-success")} />;
    case "skipped":
      return <Minus aria-label={label} className={cn(className, "text-muted-foreground")} />;
    case "failed":
      return <X aria-label={label} className={cn(className, "text-destructive")} />;
    default:
      return <Circle aria-label={label} className={cn(className, "text-faint-foreground")} />;
  }
}
