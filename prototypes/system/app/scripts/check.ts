// Lays out every diagram zoom of system.json and fails when a line crosses another line,
// runs through a box, or a label covers a box.
import { readFileSync } from "node:fs";
import { layoutDiagram, layoutProblems } from "../src/layout.ts";
import { SystemIndex } from "../src/model.ts";

const index = new SystemIndex(JSON.parse(readFileSync(new URL("../../system.json", import.meta.url), "utf8")));
let total = 0;
for (const zoom of index.zooms.filter((zoom) => zoom.view === "diagram")) {
  const problems = layoutProblems(await layoutDiagram(index, index.graph(zoom)));
  total += problems.length;
  for (const problem of problems) console.log(`${zoom.id}: ${problem.text}`);
}
console.log(total === 0 ? "Every zoom lays out cleanly." : `${total} layout problems.`);
process.exit(total === 0 ? 0 : 1);
