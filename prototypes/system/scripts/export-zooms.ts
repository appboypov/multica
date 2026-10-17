/**
 * Serves build/ in-process, walks every zoom like a reader, and screenshots each sheet to
 * exports/. Any failed step throws, so this run is also the smoke test of the build.
 */
import { existsSync, mkdirSync, rmSync } from "node:fs";
import { join, normalize } from "node:path";
import { chromium, expect, type Page } from "@playwright/test";
import { exportFileName, system } from "../src/lib/system.ts";

const root = join(import.meta.dir, "..");
const buildDir = join(root, "build");
const exportDir = join(root, "exports");

if (!existsSync(join(buildDir, "index.html"))) {
  throw new Error(`No ${join(buildDir, "index.html")}: run \`bun run build\` first`);
}

const server = Bun.serve({
  hostname: "127.0.0.1",
  port: 0,
  async fetch(request) {
    const pathname = decodeURIComponent(new URL(request.url).pathname);
    const file = Bun.file(join(buildDir, normalize(pathname === "/" ? "/index.html" : pathname)));
    return (await file.exists()) ? new Response(file) : new Response("Not found", { status: 404 });
  },
});

const expectZoom = async (page: Page, id: string) => {
  const zoom = system.zooms.find((candidate) => candidate.id === id);
  if (zoom === undefined) throw new Error(`system.json has no zoom ${id}`);
  await expect(page.getByTestId("zoom-title")).toHaveText(zoom.title);
  await expect(page.getByTestId("sheet")).toHaveAttribute("data-zoom", id);
  expect(new URL(page.url()).hash).toBe(`#/zoom/${id}`);
};

const browser = await chromium.launch();
try {
  const page = await browser.newPage({
    viewport: { width: 1600, height: 1000 },
    deviceScaleFactor: 2,
  });
  const failures: string[] = [];
  page.on("pageerror", (error) => failures.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") failures.push(message.text());
  });

  await page.goto(server.url.href);
  await expect(page.getByTestId("zoom-title")).toHaveText(system.zooms[0]?.title ?? "");
  await page.evaluate(async () => {
    await document.fonts.ready;
  });

  rmSync(exportDir, { recursive: true, force: true });
  mkdirSync(exportDir, { recursive: true });

  for (const zoom of system.zooms) {
    await page.getByTestId(`zoom-link-${zoom.id}`).click();
    await expectZoom(page, zoom.id);
    if (zoom.view === "map") {
      await expect(page.locator("[data-testid^='map-zoom-']")).toHaveCount(system.zooms.length - 1);
    } else if (zoom.view === "diagram") {
      await expect(page.locator("[data-node]")).toHaveCount(zoom.nodes.length);
      await expect(page.locator("[data-edge]")).toHaveCount(zoom.edges.length);
    } else {
      const rows = zoom.tables.reduce((count, table) => count + table.rows.length, 0);
      await expect(page.locator("[data-row]")).toHaveCount(rows);
    }
    const clipped = await page
      .getByTestId("sheet")
      .evaluate(
        (sheet) =>
          sheet.scrollWidth > sheet.clientWidth ||
          [...sheet.querySelectorAll("table")].some(
            (table) => table.offsetWidth > (table.parentElement?.clientWidth ?? 0),
          ),
      );
    if (clipped) throw new Error(`Zoom ${zoom.id} has a table wider than its column`);
    const file = join(exportDir, exportFileName(zoom));
    await page.getByTestId("sheet").screenshot({ path: file, animations: "disabled" });
    console.log(`exported ${file}`);
  }

  // Drill down from a focus node, climb back with the browser, then up through the boundary.
  await page.getByTestId("zoom-link-02").click();
  await page.getByTestId("drill-03").click();
  await expectZoom(page, "03");
  await page.goBack();
  await expectZoom(page, "02");
  await page.getByTestId("drill-03").click();
  await page.getByTestId("boundary-up").click();
  await expectZoom(page, "02");
  // The zoom map opens the deepest zoom straight from its box label.
  await page.getByTestId("zoom-link-00").click();
  await page.getByTestId("map-zoom-04").click();
  await expectZoom(page, "04");

  if (failures.length > 0) throw new Error(`The page logged errors:\n${failures.join("\n")}`);
  console.log(`smoke passed: ${system.zooms.length} zooms, drill-down, back, boundary up`);
} finally {
  await browser.close();
  await server.stop(true);
}
