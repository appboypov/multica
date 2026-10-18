import type { SystemIndex, Zoom } from "./model.ts";
import type { JourneyStep } from "./system.ts";

/** What a journey step lights up on a zoom. */
export interface StepFocus {
  /** The edge of system.json the step's data moves over. */
  edge: string | null;
  /** Parts this zoom draws that the step touches. */
  parts: Set<string>;
  /** Models the step reads or writes. */
  models: Set<string>;
  /** The zoom where the step runs between two parts side by side. */
  finest: Zoom;
}

export function stepFocus(index: SystemIndex, step: JourneyStep, zoom: Zoom): StepFocus {
  const opened = zoom.part?.id ?? null;
  const edge = step.edge ? index.design.edges.find((candidate) => candidate.id === step.edge) : undefined;
  const ends = edge ? [edge.from, edge.to] : step.part ? [step.part] : [];
  const parts = new Set(ends.map((id) => index.representative(id, opened).part.id));
  const models = new Set([...(step.in ?? []), ...(step.out ?? [])].map((ref) => ref.model));
  const finest = edge ? index.edgeZoom(edge) : step.part ? index.zoomOf(step.part) : zoom;
  return { edge: edge?.id ?? null, parts, models, finest };
}
