import type { LineStyle } from "./system.ts";

/** How the template draws each line style: solid call, dashed event, dotted data. */
export const lineStroke: Record<LineStyle, { width: number; dash: string; cap: "butt" | "round" }> =
  {
    call: { width: 1.5, dash: "none", cap: "butt" },
    event: { width: 1.5, dash: "4 3", cap: "butt" },
    data: { width: 2, dash: "0.1 4", cap: "round" },
  };
