import type { Edge, LineType, Model, Part, SystemDesign, ZoomInfo } from "./system.ts";

/** One sheet of the prototype. */
export interface Zoom {
  /** "map", "world" or the id of the part the zoom opens. */
  id: string;
  /** The part the zoom opens; null for the map and the world. */
  part: Part | null;
  level: number;
  info: ZoomInfo;
  view: "map" | "diagram" | "paths" | "models";
}

/** A line as one zoom draws it, between the parts that zoom shows. */
export interface ZoomEdge {
  id: string;
  from: string;
  to: string;
  line: LineType;
  /** The line also carries edges from `to` back to `from`. */
  both: boolean;
  /** The edges of system.json this line stands for. */
  edges: Edge[];
  label: string;
  /** The full label when `label` is shortened. */
  title: string;
}

export interface ZoomGraph {
  zoom: Zoom;
  /** The parts inside the opened part; empty for the world. */
  inside: Part[];
  /** The world's parts, or the parts outside the opened part it talks to. */
  around: Part[];
  edges: ZoomEdge[];
}

export const MAP_ZOOM_ID = "map";
export const WORLD_ZOOM_ID = "world";

export class SystemIndex {
  readonly design: SystemDesign;
  readonly parts = new Map<string, Part>();
  readonly children = new Map<string | null, Part[]>();
  readonly models = new Map<string, Model>();
  readonly zooms: Zoom[] = [];
  readonly zoomById = new Map<string, Zoom>();
  readonly maxLevel: number;

  constructor(design: SystemDesign) {
    this.design = design;
    for (const part of design.parts) {
      this.parts.set(part.id, part);
      const siblings = this.children.get(part.parent) ?? [];
      siblings.push(part);
      this.children.set(part.parent, siblings);
    }
    for (const model of design.models) this.models.set(model.name, model);

    this.addZoom({
      id: MAP_ZOOM_ID,
      part: null,
      level: 0,
      view: "map",
      info: {
        name: "Zoom map",
        title: "Zoom map",
        description: "Each zoom opens one part of the zoom above it",
        question: "Which zoom holds the part I am looking for?",
      },
    });
    this.addZoom({ id: WORLD_ZOOM_ID, part: null, level: 1, view: "diagram", info: design.world });
    const visit = (parent: string | null, level: number) => {
      for (const part of this.children.get(parent) ?? []) {
        if (part.zoom) {
          this.addZoom({ id: part.id, part, level, info: part.zoom, view: this.viewOf(part) });
        }
        visit(part.id, level + 1);
      }
    };
    visit(null, 2);
    this.maxLevel = Math.max(...this.zooms.map((zoom) => zoom.level));
  }

  private addZoom(zoom: Zoom) {
    this.zooms.push(zoom);
    this.zoomById.set(zoom.id, zoom);
  }

  private viewOf(part: Part): Zoom["view"] {
    if ((this.children.get(part.id) ?? []).length > 0) return part.zoom?.view ?? "diagram";
    return "models";
  }

  childrenOf(id: string | null): Part[] {
    return this.children.get(id) ?? [];
  }

  modelsOf(owner: string): Model[] {
    return this.design.models.filter((model) => model.owner === owner);
  }

  /** The part, then its parent, up to the part in the world. */
  chain(id: string): Part[] {
    const chain: Part[] = [];
    let part = this.parts.get(id);
    while (part) {
      chain.push(part);
      part = part.parent ? this.parts.get(part.parent) : undefined;
    }
    return chain;
  }

  /** The zoom that opens the part, or the zoom its parent opens. */
  zoomOf(id: string): Zoom {
    const part = this.parts.get(id);
    if (!part?.parent) return this.zoomById.get(WORLD_ZOOM_ID)!;
    return this.zoomById.get(part.parent)!;
  }

  /**
   * The part a zoom draws for `id`: its ancestor among the opened part's
   * children, or, outside the opened part, its ancestor at the level where
   * its branch leaves the opened part's chain.
   */
  representative(id: string, opened: string | null): { part: Part; inside: boolean } {
    const chain = this.chain(id);
    if (opened === null) return { part: chain[chain.length - 1], inside: true };
    const inside = chain.find((part) => part.parent === opened);
    if (inside) return { part: inside, inside: true };
    const openedChain = new Set<string | null>([null, ...this.chain(opened).map((part) => part.id)]);
    const around = chain.find((part) => openedChain.has(part.parent) && part.id !== opened);
    return { part: around ?? chain[chain.length - 1], inside: false };
  }

  /** The zoom where an edge runs between two parts side by side: the zoom of their nearest common part. */
  edgeZoom(edge: Edge): Zoom {
    const fromChain = new Set(this.chain(edge.from).map((part) => part.id));
    const common = this.chain(edge.to).find((part) => fromChain.has(part.id));
    return common ? (this.zoomById.get(common.id) ?? this.zoomOf(common.id)) : this.zoomById.get(WORLD_ZOOM_ID)!;
  }

  graph(zoom: Zoom): ZoomGraph {
    const opened = zoom.part?.id ?? null;
    const insideIds = new Set(this.childrenOf(opened).map((part) => part.id));
    const around = new Map<string, Part>();
    const merged = new Map<string, ZoomEdge>();

    for (const edge of this.design.edges) {
      const from = this.representative(edge.from, opened);
      const to = this.representative(edge.to, opened);
      if (from.part.id === to.part.id) continue;
      if (opened !== null && !from.inside && !to.inside) continue;
      if (!from.inside) around.set(from.part.id, from.part);
      if (!to.inside) around.set(to.part.id, to.part);
      const key = `${from.part.id}>${to.part.id}>${edge.line}`;
      const line = merged.get(key);
      if (line) line.edges.push(edge);
      else {
        merged.set(key, {
          id: key,
          from: from.part.id,
          to: to.part.id,
          line: edge.line,
          both: false,
          edges: [edge],
          label: "",
          title: "",
        });
      }
    }

    // In a diagram, data and event lines that two or more parts share with one outside part meet the boundary as one line.
    if (opened !== null && zoom.view === "diagram") {
      const shared = new Map<string, ZoomEdge[]>();
      for (const line of merged.values()) {
        if (line.line === "call" || (!around.has(line.from) && !around.has(line.to))) continue;
        const key = around.has(line.to) ? `${opened}>${line.to}>${line.line}` : `${line.from}>${opened}>${line.line}`;
        shared.set(key, [...(shared.get(key) ?? []), line]);
      }
      for (const [key, lines] of shared) {
        if (lines.length < 2) continue;
        for (const line of lines) merged.delete(line.id);
        const [from, to] = key.split(">");
        merged.set(key, { ...lines[0], id: key, from, to, edges: lines.flatMap((line) => line.edges) });
      }
    }

    // Lines of one kind between the same two parts in both directions draw as one two-way line.
    for (const line of merged.values()) {
      const back = merged.get(`${line.to}>${line.from}>${line.line}`);
      if (!back || line.both) continue;
      line.edges.push(...back.edges);
      line.both = true;
      merged.delete(back.id);
    }

    for (const line of merged.values()) {
      const { label, title } = labelOf(line.edges);
      line.label = label;
      line.title = title;
    }

    const inside = opened === null ? [] : this.childrenOf(opened);
    const aroundParts =
      opened === null
        ? this.childrenOf(null)
        : this.design.parts.filter((part) => around.has(part.id) && !insideIds.has(part.id));
    return { zoom, inside, around: aroundParts, edges: [...merged.values()] };
  }
}

const MAX_LABEL_ITEMS = 2;

function joinUnique(values: string[]): { short: string; full: string } {
  const unique = [...new Set(values)];
  const full = unique.join(", ");
  if (unique.length <= MAX_LABEL_ITEMS) return { short: full, full };
  return {
    short: `${unique.slice(0, MAX_LABEL_ITEMS).join(", ")} +${unique.length - MAX_LABEL_ITEMS}`,
    full,
  };
}

function labelOf(edges: Edge[]): { label: string; title: string } {
  const data = joinUnique(edges.map((edge) => edge.data));
  const trigger = joinUnique(edges.map((edge) => edge.trigger));
  return { label: `${data.short} · ${trigger.short}`, title: `${data.full} · ${trigger.full}` };
}

/** The path shown on a sheet: the part's path without the sheet's base. */
export function shownPath(part: Part, base: string | undefined): string | undefined {
  if (!part.path) return part.note;
  if (base && part.path.startsWith(base) && part.path.length > base.length) {
    return part.path.slice(base.length);
  }
  return part.path;
}

export function zoomNumber(level: number): string {
  return String(level).padStart(2, "0");
}
