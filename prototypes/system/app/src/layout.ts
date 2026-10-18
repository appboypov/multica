import ELK, { type ElkExtendedEdge, type ElkNode } from "elkjs/lib/elk.bundled.js";
import type { SystemIndex, ZoomEdge, ZoomGraph } from "./model.ts";
import type { Part } from "./system.ts";

/**
 * Sizes of the template's parts. Node heights follow from the template's
 * paddings and line heights, so the layout and the rendered boxes agree
 * without measuring the DOM.
 */
export const SIZE = {
  node: 232,
  focus: 272,
  /** Shown "Opens into" rows; more fold into "+N more". */
  focusRows: 6,
  chip: 22,
  boundaryPad: 32,
  boundaryTop: 48,
} as const;

export interface Point {
  x: number;
  y: number;
}

export interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface PlacedPart extends Box {
  part: Part;
  variant: "node" | "outside" | "focus";
  /** "Opens into" rows of a focus node. */
  opens: string[];
  /** The part sits outside the opened part. */
  around: boolean;
}

export interface PlacedEdge {
  edge: ZoomEdge;
  points: Point[];
  label: Box;
}

export interface PlacedChips extends Box {
  parts: Part[];
  columns: Part[][];
}

export interface DiagramLayout {
  width: number;
  height: number;
  parts: PlacedPart[];
  edges: PlacedEdge[];
  boundary: (Box & { label: string; labelX: number }) | null;
  chips: PlacedChips | null;
}

const CHIPS_ID = "__chips";
const CHIPS_PER_COLUMN = 6;
/** Partitions of the rows outside parts sit in, around the boundary's ports (2 and 4) and the inside parts (3). */
const ROW = { farAbove: 0, above: 1, below: 5, farBelow: 6 } as const;

// Approximate advance widths of Inter at 1px, by character class.
const NARROW = new Set([..."iljtfI.,:;'|!·()[] "]);
const WIDE = new Set([..."mwMW@"]);

/** Width of a line of Inter text, close enough to size pills and chips. */
export function textWidth(text: string, size: number, weight: 400 | 500 | 600): number {
  let units = 0;
  for (const char of text) {
    if (NARROW.has(char)) units += 0.3;
    else if (WIDE.has(char)) units += 0.86;
    else if (char >= "A" && char <= "Z") units += 0.68;
    else units += 0.56;
  }
  return Math.ceil(units * size * (weight === 600 ? 1.04 : weight === 500 ? 1.02 : 1));
}

/** Pill of the edge label or chip: 12/16 text, padding 2px 8px, 1px border. */
export function pillWidth(text: string, weight: 500 | 600 = 500): number {
  return textWidth(text, 12, weight) + 18;
}

function opensInto(index: SystemIndex, part: Part): string[] {
  const names = index.childrenOf(part.id).map((child) => child.name);
  if (names.length === 0) {
    const models = index.modelsOf(part.id).length;
    return models > 0 ? [`${models} models`] : [];
  }
  if (names.length <= SIZE.focusRows) return names;
  return [...names.slice(0, SIZE.focusRows - 1), `+${names.length - SIZE.focusRows + 1} more`];
}

/** Height of a node: header 16 + 20 (+ 4 + 18 path), content 12 + badges 20 + 16, borders. */
export function nodeHeight(part: Part, variant: PlacedPart["variant"], opens: number): number {
  const border = variant === "focus" ? 3 : 2;
  const header = 16 + 20 + (part.path || part.note ? 22 : 0);
  const parts = variant === "focus" ? 1 + 12 + 16 + opens * 24 + 8 : 0;
  return border + header + 12 + parts + 20 + 16;
}

function chipColumns(parts: Part[]): Part[][] {
  const columns: Part[][] = [];
  for (let i = 0; i < parts.length; i += CHIPS_PER_COLUMN) {
    columns.push(parts.slice(i, i + CHIPS_PER_COLUMN));
  }
  return columns;
}

const elk = new ELK();

export async function layoutDiagram(index: SystemIndex, graph: ZoomGraph): Promise<DiagramLayout> {
  const opened = graph.zoom.part;
  // A part is on the branch when any of its edges is drawn, also inside a line that meets the boundary.
  const connected = new Set(
    graph.edges.flatMap((line) =>
      line.edges.flatMap((edge) => [edge.from, edge.to].map((id) => index.representative(id, opened?.id ?? null).part.id)),
    ),
  );
  const drawn = opened ? graph.inside.filter((part) => connected.has(part.id)) : graph.around;
  const offBranch = opened ? graph.inside.filter((part) => !connected.has(part.id)) : [];

  const placed = new Map<string, PlacedPart>();
  const elkNode = (part: Part, inside: boolean, layoutOptions: Record<string, string>): ElkNode => {
    const variant: PlacedPart["variant"] = part.outside
      ? "outside"
      : inside && index.zoomById.has(part.id)
        ? "focus"
        : "node";
    const opens = variant === "focus" ? opensInto(index, part) : [];
    const width = variant === "focus" ? SIZE.focus : SIZE.node;
    const height = nodeHeight(part, variant, opens.length);
    placed.set(part.id, { part, variant, opens, around: !inside, x: 0, y: 0, width, height });
    return { id: part.id, width, height, layoutOptions };
  };

  const columns = chipColumns(offBranch);
  const chipsNode: ElkNode | null =
    offBranch.length > 0
      ? {
          id: CHIPS_ID,
          width: Math.max(
            textWidth("Also inside, not on this branch", 12, 500),
            columns.reduce(
              (sum, column) => sum + Math.max(...column.map((part) => pillWidth(part.name))) + 8,
              -8,
            ),
          ),
          height: 16 + 8 + Math.min(offBranch.length, CHIPS_PER_COLUMN) * (SIZE.chip + 6) - 6,
        }
      : null;

  // Child order decides which way a cycle of inside parts runs: a line from a later child to an earlier one is laid out backwards.
  const order = new Map(drawn.map((part, i) => [part.id, i]));
  const backward = (edge: ZoomEdge) => (order.get(edge.from) ?? -1) > (order.get(edge.to) ?? Infinity);

  // A line that meets the boundary ends at a port: a point the boundary passes through.
  // Partitions run top to bottom: the far row and the row above, ports on the boundary's top, parts inside,
  // ports on its bottom, and the row and the far row below. `rows` holds the partition of each outside part.
  const build = (rows: Map<string, number>, partitioned: boolean): ElkNode => {
    const partition = (value: number): Record<string, string> =>
      partitioned ? { "elk.partitioning.partition": String(value) } : {};
    const ports: ElkNode[] = [];
    const end = (edge: ZoomEdge, id: string, other: string) => {
      if (id !== opened?.id) return id;
      ports.push({ id: `${edge.id}:port`, width: 1, height: 0, layoutOptions: partition(rows.get(other)! < 3 ? 2 : 4) });
      return `${edge.id}:port`;
    };
    const edges: ElkExtendedEdge[] = graph.edges
      .filter((edge) => partitioned || (edge.from !== opened?.id && edge.to !== opened?.id))
      .map((edge) => {
        const ends = [end(edge, edge.from, edge.to), end(edge, edge.to, edge.from)];
        if (backward(edge)) ends.reverse();
        return {
          id: edge.id,
          sources: [ends[0]],
          targets: [ends[1]],
          labels: [{ id: `${edge.id}:label`, text: edge.label, width: pillWidth(edge.label), height: 20 }],
        };
      });
    const outsideNodes = opened ? graph.around.map((part) => elkNode(part, false, partition(rows.get(part.id)!))) : [];
    const insideNodes = drawn.map((part) => elkNode(part, true, partition(3)));
    if (chipsNode) insideNodes.push({ ...chipsNode, layoutOptions: partition(3) });
    return {
      id: "root",
      layoutOptions: {
        "elk.algorithm": "layered",
        "elk.direction": "DOWN",
        "elk.edgeRouting": "ORTHOGONAL",
        "elk.partitioning.activate": String(partitioned),
        "elk.separateConnectedComponents": "false",
        "elk.layered.crossingMinimization.strategy": "LAYER_SWEEP",
        "elk.layered.crossingMinimization.greedySwitch.type": "TWO_SIDED",
        "elk.layered.nodePlacement.strategy": "NETWORK_SIMPLEX",
        "elk.layered.considerModelOrder.strategy": "NODES_AND_EDGES",
        "elk.layered.thoroughness": "100",
        "elk.spacing.nodeNode": "56",
        "elk.layered.spacing.nodeNodeBetweenLayers": "72",
        "elk.spacing.edgeNode": "28",
        "elk.layered.spacing.edgeNodeBetweenLayers": "28",
        "elk.spacing.edgeEdge": "20",
        "elk.layered.spacing.edgeEdgeBetweenLayers": "20",
        "elk.spacing.edgeLabel": "4",
        "elk.edgeLabels.inline": "true",
        "elk.edgeLabels.placement": "CENTER",
        "elk.layered.mergeEdges": "false",
        "elk.padding": "[top=8,left=8,bottom=8,right=8]",
      },
      children: [...outsideNodes, ...ports, ...insideNodes],
      edges,
    };
  };

  // An outside part sits above the opened part when a free layout puts it in the upper half,
  // else, with no line of its own in that layout, when most of its lines go into the opened part.
  const sendsMost = (id: string) =>
    graph.edges.filter((edge) => edge.from === id).length > graph.edges.filter((edge) => edge.to === id).length;
  let rows = new Map<string, number>(graph.around.map((part) => [part.id, sendsMost(part.id) ? ROW.above : ROW.below]));
  if (opened) {
    const free = await elk.layout(build(rows, false));
    const middle = new Map((free.children ?? []).map((child) => [child.id, (child.y ?? 0) + (child.height ?? 0) / 2]));
    const insideMiddles = drawn.map((part) => middle.get(part.id)!);
    const split = (Math.min(...insideMiddles) + Math.max(...insideMiddles)) / 2;
    const linked = new Set((free.edges ?? []).flatMap((edge) => [...edge.sources, ...edge.targets]));
    for (const id of linked) if (rows.has(id)) rows.set(id, middle.get(id)! < split ? ROW.above : ROW.below);
  }

  const byId = new Map(graph.edges.map((edge) => [edge.id, edge]));
  const place = async (rows: Map<string, number>): Promise<DiagramLayout> => {
    const result = await elk.layout(build(rows, true));

    let chips: PlacedChips | null = null;
    const portBoxes: Box[] = [];
    for (const child of result.children ?? []) {
      const box = { x: child.x ?? 0, y: child.y ?? 0, width: child.width ?? 0, height: child.height ?? 0 };
      if (child.id === CHIPS_ID) chips = { ...box, parts: offBranch, columns };
      else if (child.id.endsWith(":port")) portBoxes.push(box);
      else Object.assign(placed.get(child.id)!, box);
    }

    const insideBoxes: Box[] = [...drawn.map((part) => placed.get(part.id)!), ...(chips ? [chips] : [])];
    const left = Math.min(...[...insideBoxes, ...portBoxes].map((box) => box.x)) - SIZE.boundaryPad;
    const right = Math.max(...[...insideBoxes, ...portBoxes].map((box) => box.x + box.width)) + SIZE.boundaryPad;
    const top = Math.min(...insideBoxes.map((box) => box.y - SIZE.boundaryTop), ...portBoxes.map((box) => box.y));
    const bottom = Math.max(
      ...insideBoxes.map((box) => box.y + box.height + SIZE.boundaryPad),
      ...portBoxes.map((box) => box.y),
    );

    const placedEdges: PlacedEdge[] = (result.edges ?? []).map((edge) => {
      const section = edge.sections![0];
      const points = [section.startPoint, ...(section.bendPoints ?? []), section.endPoint];
      if (backward(byId.get(edge.id)!)) points.reverse();
      const label = edge.labels![0];
      return {
        edge: byId.get(edge.id)!,
        points,
        label: {
          x: label.x ?? 0,
          y: label.y ?? 0,
          width: label.width ?? 0,
          height: label.height ?? 0,
        },
      };
    });

    // The boundary label sits on the top edge at the first place no line or label covers.
    const labelText = opened ? `${opened.name} · from zoom ${String(index.zoomOf(opened.id).level).padStart(2, "0")}` : "";
    const labelBox = (offset: number): Box => ({ x: left + offset, y: top - 11, width: pillWidth(labelText, 600), height: 20 });
    const free = (offset: number) =>
      placedEdges.every(
        (placed) =>
          !overlaps(placed.label, labelBox(offset)) &&
          placed.points.slice(1).every((end, i) => !segmentHitsBox(placed.points[i], end, labelBox(offset))),
      );
    const offsets = [24, ...placedEdges.flatMap((placed) => placed.points.map((point) => point.x - left + 16))].filter(
      (offset) => offset >= 24 && offset + labelBox(0).width <= right - left - 24,
    );
    const boundary: DiagramLayout["boundary"] = opened
      ? {
          x: left,
          y: top,
          width: right - left,
          height: bottom - top,
          label: labelText,
          labelX: offsets.sort((a, b) => a - b).find(free) ?? 24,
        }
      : null;

    return {
      width: result.width ?? 0,
      height: result.height ?? 0,
      parts: [...placed.values()],
      edges: placedEdges,
      boundary,
      chips,
    };
  };

  // An outside part on a line with a problem moves to another row when that leaves fewer problems.
  let layout = await place(rows);
  let problems = layoutProblems(layout);
  for (let moved = true; moved && problems.length > 0; ) {
    moved = false;
    const suspects = new Set(problems.flatMap((problem) => problem.lines.flatMap((line) => [line.from, line.to])));
    for (const id of [...suspects].filter((id) => rows.has(id))) {
      for (const row of Object.values(ROW).filter((row) => row !== rows.get(id))) {
        const tried = new Map(rows).set(id, row);
        const next = await place(tried);
        const nextProblems = layoutProblems(next);
        if (nextProblems.length >= problems.length) continue;
        [rows, layout, problems, moved] = [tried, next, nextProblems, true];
        break;
      }
      if (moved) break;
    }
  }
  return layout;
}

export interface LayoutProblem {
  text: string;
  /** Lines the problem is on. */
  lines: ZoomEdge[];
}

/** Lines that cross another line, run through a box, or labels that cover a box. */
export function layoutProblems(layout: DiagramLayout): LayoutProblem[] {
  const problems = new Map<string, ZoomEdge[]>();
  const add = (text: string, ...lines: ZoomEdge[]) => problems.set(text, lines);
  const segments = layout.edges.flatMap((placed) =>
    placed.points.slice(1).map((end, i) => ({ placed, start: placed.points[i], end })),
  );

  for (let i = 0; i < segments.length; i++) {
    for (let j = i + 1; j < segments.length; j++) {
      const a = segments[i];
      const b = segments[j];
      if (a.placed === b.placed) continue;
      if (crosses(a.start, a.end, b.start, b.end)) {
        add(`line ${a.placed.edge.id} crosses ${b.placed.edge.id}`, a.placed.edge, b.placed.edge);
      }
    }
  }

  const boxes: (Box & { id: string })[] = [
    ...layout.parts.map((part) => ({ ...part, id: part.part.id })),
    ...(layout.chips ? [{ ...layout.chips, id: CHIPS_ID }] : []),
    ...(layout.boundary
      ? [
          {
            id: "boundary label",
            x: layout.boundary.x + layout.boundary.labelX,
            y: layout.boundary.y - 11,
            width: pillWidth(layout.boundary.label, 600),
            height: 20,
          },
        ]
      : []),
  ];
  for (const segment of segments) {
    for (const box of boxes) {
      if (box.id === segment.placed.edge.from || box.id === segment.placed.edge.to) continue;
      if (segmentHitsBox(segment.start, segment.end, box)) {
        add(`line ${segment.placed.edge.id} runs through ${box.id}`, segment.placed.edge);
      }
    }
  }
  for (const placed of layout.edges) {
    for (const box of boxes) {
      if (overlaps(placed.label, box)) add(`label of ${placed.edge.id} covers ${box.id}`, placed.edge);
    }
  }
  for (const part of layout.parts) {
    if (part.around && layout.boundary && overlaps(part, layout.boundary)) {
      const lines = layout.edges.filter((placed) => [placed.edge.from, placed.edge.to].includes(part.part.id));
      add(`${part.part.id} sits inside the boundary`, ...lines.map((placed) => placed.edge));
    }
  }
  return [...problems].map(([text, lines]) => ({ text, lines }));
}

function crosses(a1: Point, a2: Point, b1: Point, b2: Point): boolean {
  const aVertical = a1.x === a2.x;
  const bVertical = b1.x === b2.x;
  if (aVertical === bVertical) return false;
  const [v1, v2, h1, h2] = aVertical ? [a1, a2, b1, b2] : [b1, b2, a1, a2];
  const x = v1.x;
  const y = h1.y;
  const inH = x > Math.min(h1.x, h2.x) + 0.5 && x < Math.max(h1.x, h2.x) - 0.5;
  const inV = y > Math.min(v1.y, v2.y) + 0.5 && y < Math.max(v1.y, v2.y) - 0.5;
  return inH && inV;
}

function segmentHitsBox(start: Point, end: Point, box: Box): boolean {
  const left = Math.min(start.x, end.x);
  const right = Math.max(start.x, end.x);
  const top = Math.min(start.y, end.y);
  const bottom = Math.max(start.y, end.y);
  return right > box.x + 1 && left < box.x + box.width - 1 && bottom > box.y + 1 && top < box.y + box.height - 1;
}

function overlaps(a: Box, b: Box): boolean {
  return a.x < b.x + b.width && a.x + a.width > b.x && a.y < b.y + b.height && a.y + a.height > b.y;
}
