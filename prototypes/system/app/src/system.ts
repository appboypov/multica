/**
 * The shape of `prototypes/system/system.json`, the one source of Multica's
 * living system design. Every page of the prototype renders from it.
 */

export type LineType = "call" | "event" | "data";

/** What a zoom sheet shows in its header and sidebar. */
export interface ZoomInfo {
  /** Sidebar label. */
  name: string;
  /** Header title. */
  title: string;
  /** Header line under the title. */
  description: string;
  /** The question the sheet answers, shown at the foot of the sidebar. */
  question: string;
  /** Path prefix left out of the part paths shown on this sheet. */
  base?: string;
  /** "paths" draws the parts as a call path table instead of a diagram. */
  view?: "diagram" | "paths";
}

/**
 * A box on the design. A part with children, or one that owns models, opens
 * into its own zoom.
 */
export interface Part {
  id: string;
  /** The part it sits inside; null for the parts in the product's world. */
  parent: string | null;
  name: string;
  /** Lucide icon name in kebab case. */
  icon: string;
  /** Badge text, such as Service, Module, Store or Function. */
  kind: string;
  /** People and systems that are not Multica's own code. */
  outside?: boolean;
  /** Repository path; a leaf part's path ends in `:line`. */
  path?: string;
  /** Short mono line shown when the part has no path. */
  note?: string;
  /** Required when the part has children or owns models. */
  zoom?: ZoomInfo;
}

/**
 * One line between two parts, named by the data it carries and what starts
 * it. Edges sit between the finest parts known; a zoom draws them between the
 * parts it shows.
 */
export interface Edge {
  id: string;
  from: string;
  to: string;
  line: LineType;
  data: string;
  trigger: string;
  /** Name of the event in `events` that the edge carries. */
  event?: string;
}

export interface SystemEvent {
  /** Wire name, such as `task:queued`. */
  name: string;
  /** Declaration, `file:line`. */
  path: string;
}

export interface ModelField {
  name: string;
  type: string;
}

export interface ModelRelation {
  /** Name of the related model. */
  to: string;
  /** Cardinality on this model's side, such as "1" or "*". */
  from: string;
  /** Cardinality on the related model's side. */
  toCount: string;
  /** Column or field that holds the relation. */
  via: string;
  /** A logical reference without a foreign key. */
  soft?: boolean;
}

export interface Model {
  name: string;
  kind: "table" | "contract";
  /** Part that owns the model; its zoom lists it. */
  owner: string;
  /** Declaration, `file:line`. */
  path: string;
  fields: ModelField[];
  relations?: ModelRelation[];
}

/** Data shown on a journey step, in the shape of a model. */
export interface DataRef {
  model: string;
  /** Fields of the model the step reads or writes; all when absent. */
  fields?: string[];
}

export interface JourneyStep {
  title: string;
  /** What happens in this step. */
  happens: string;
  /** The edge the step's data moves over. */
  edge?: string;
  /** The part that acts when the step moves over no edge. */
  part?: string;
  in?: DataRef[];
  out?: DataRef[];
  /** Where the step stores its data. */
  store?: string;
  /** What the agent does in this step. */
  action?: string;
}

export interface Journey {
  id: string;
  name: string;
  /** The question the journey answers. */
  question: string;
  steps: JourneyStep[];
}

export interface SystemDesign {
  product: {
    name: string;
    /** The code the design is derived from, such as "Multica fork · 39bbbce". */
    source: string;
  };
  /** Zoom 01: the product in its world. */
  world: ZoomInfo;
  parts: Part[];
  edges: Edge[];
  events: SystemEvent[];
  models: Model[];
  journeys: Journey[];
}
