import { z } from "zod";
import raw from "../../system.json";

/** A zoom number as the sheet shows it: two digits, 00 is the zoom map. */
const ZoomId = z.string().regex(/^\d{2}$/);
const Id = z.string().regex(/^[a-z0-9-]+$/);
const Text = z.string().min(1);
/** A repo-relative file or folder, with an optional line: never a caption. */
const RepoPath = z.string().regex(/^[\w.@+-]+(\/[\w.@+-]+)*\/?(:\d+)?$/);
const Point = z.tuple([z.number(), z.number()]);
const Size = z.int().nonnegative();

export const LineStyle = z.enum(["call", "event", "data"]);
export type LineStyle = z.infer<typeof LineStyle>;

export const IconName = z.enum([
  "app-window",
  "box",
  "camera",
  "database",
  "door-open",
  "file-code",
  "flask-conical",
  "folder",
  "globe",
  "list",
  "list-todo",
  "map",
  "map-pin",
  "mouse-pointer-click",
  "package",
  "play",
  "scroll-text",
  "send",
  "server",
  "settings",
  "shapes",
  "tag",
  "terminal",
  "user",
  "workflow",
  "zap",
]);
export type IconName = z.infer<typeof IconName>;

const Box = { x: Size, y: Size, w: Size, h: Size };

export const Node = z
  .strictObject({
    id: Id,
    kind: z.enum(["node", "focus", "outside"]),
    name: Text,
    icon: IconName,
    path: RepoPath.optional(),
    caption: Text.optional(),
    badge: Text,
    opens: ZoomId.optional(),
    parts: z.array(Text).min(1).optional(),
    ...Box,
  })
  .refine((node) => (node.path === undefined) !== (node.caption === undefined), {
    message: "A node has a repo path or a caption, not both",
  })
  .refine(
    (node) =>
      (node.kind === "focus") === (node.opens !== undefined) &&
      (node.kind === "focus") === (node.parts !== undefined),
    { message: "Only a focus node opens into a zoom and lists its parts" },
  );
export type Node = z.infer<typeof Node>;

export const Edge = z.strictObject({
  id: Id,
  from: Id,
  to: Id,
  style: LineStyle,
  data: Text,
  trigger: Text,
  points: z.array(Point).min(2),
  label: Point,
});
export type Edge = z.infer<typeof Edge>;

export const Boundary = z.strictObject({ label: Text, up: ZoomId, ...Box });
export type Boundary = z.infer<typeof Boundary>;

export const CallsTable = z.strictObject({
  kind: z.literal("calls"),
  title: Text,
  hint: Text,
  summary: Text,
  columns: z.tuple([Text, Text, Text]),
  rows: z
    .array(
      z.strictObject({
        name: Text,
        indent: z.int().min(0).max(3),
        path: RepoPath,
        out: z.strictObject({ style: LineStyle, text: Text }).optional(),
      }),
    )
    .min(1),
});
export type CallsTable = z.infer<typeof CallsTable>;

/** Rows of plain facts; the last column is the repo path the fact comes from. */
export const FactsTable = z
  .strictObject({
    kind: z.literal("facts"),
    title: Text,
    hint: Text,
    summary: Text,
    columns: z.array(Text).min(2),
    rows: z.array(z.array(Text)).min(1),
  })
  .refine(
    (table) =>
      table.rows.every(
        (row) => row.length === table.columns.length && RepoPath.safeParse(row.at(-1)).success,
      ),
    { message: "Every facts row fills each column and ends in a repo path" },
  );
export type FactsTable = z.infer<typeof FactsTable>;

const ZoomBase = {
  id: ZoomId,
  title: Text,
  description: Text,
  icon: IconName,
  parent: ZoomId.nullable(),
};

export const MapZoom = z.strictObject({ ...ZoomBase, view: z.literal("map") });
export const DiagramZoom = z.strictObject({
  ...ZoomBase,
  view: z.literal("diagram"),
  pathRoot: z.string(),
  height: z.int().positive(),
  nodes: z.array(Node).min(1),
  edges: z.array(Edge),
  boundaries: z.array(Boundary),
});
export const PathsZoom = z.strictObject({
  ...ZoomBase,
  view: z.literal("paths"),
  pathRoot: z.string(),
  tables: z.array(z.union([CallsTable, FactsTable])).min(1),
});

export const Zoom = z.discriminatedUnion("view", [MapZoom, DiagramZoom, PathsZoom]);
export type Zoom = z.infer<typeof Zoom>;
export type DiagramZoom = z.infer<typeof DiagramZoom>;
export type PathsZoom = z.infer<typeof PathsZoom>;

export const SystemDesign = z
  .strictObject({
    product: Text,
    description: Text,
    question: Text,
    source: Text,
    zooms: z.array(Zoom).min(2),
  })
  .superRefine((design, context) => {
    const ids = new Set(design.zooms.map((zoom) => zoom.id));
    const fail = (message: string) => context.addIssue({ code: "custom", message });
    if (ids.size !== design.zooms.length) fail("Zoom ids are unique");
    if (design.zooms[0]?.view !== "map") fail("The first zoom is the zoom map");
    for (const zoom of design.zooms) {
      if (zoom.parent !== null && !ids.has(zoom.parent))
        fail(`${zoom.id}: no parent ${zoom.parent}`);
      if (zoom.view !== "diagram") continue;
      const nodes = new Set(zoom.nodes.map((node) => node.id));
      for (const node of zoom.nodes) {
        if (node.opens !== undefined && !ids.has(node.opens))
          fail(`${node.id}: no zoom ${node.opens}`);
      }
      for (const edge of zoom.edges) {
        if (!nodes.has(edge.from) || !nodes.has(edge.to))
          fail(`${zoom.id}: edge ${edge.id} dangles`);
      }
      for (const boundary of zoom.boundaries) {
        if (boundary.up !== zoom.parent) fail(`${zoom.id}: boundary goes up to ${boundary.up}`);
      }
    }
  });
export type SystemDesign = z.infer<typeof SystemDesign>;

/** The parsed system.json: the one source every zoom, link and export reads. */
export const system: SystemDesign = SystemDesign.parse(raw);

export const lastZoomId = system.zooms.at(-1)?.id ?? "00";

export function zoomById(id: string): Zoom | undefined {
  return system.zooms.find((zoom) => zoom.id === id);
}

/** The focus node in the parent zoom that opens into this zoom. */
export function openerOf(zoom: Zoom): Node | undefined {
  const parent = zoom.parent === null ? undefined : zoomById(zoom.parent);
  if (parent?.view !== "diagram") return undefined;
  return parent.nodes.find((node) => node.opens === zoom.id);
}

export function zoomHref(id: string): string {
  return `#/zoom/${id}`;
}

/** The zoom a location hash names, or the zoom map for any other hash. */
export function zoomIdFromHash(hash: string): string {
  const id = /^#\/zoom\/(\d{2})$/.exec(hash)?.[1];
  return id !== undefined && zoomById(id) !== undefined ? id : "00";
}

export function displayPath(path: string, root: string): string {
  return root !== "" && path.startsWith(root) ? path.slice(root.length) : path;
}

export function exportFileName(zoom: Zoom): string {
  const slug = zoom.title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
  return `zoom-${zoom.id}-${slug}.png`;
}
