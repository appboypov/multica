import { expect, test, type Page } from "@playwright/test";
import { buildSurfaceFrameDocument } from "../packages/views/plugins/surface-document";

/**
 * Real Chromium coverage for the hosted-frame wrapper's terminal signals, which
 * jsdom cannot exercise: a cross-origin guest document loading, failing and
 * navigating inside the wrapper's sandboxed child frame.
 */

const CONTENT_ORIGIN = "https://plugin-content.example.test";

type SurfaceSignals = { errors: number; navigated: number };

declare global {
  interface Window {
    /** Terminal signals the test host page counts from the wrapper. */
    signals?: SurfaceSignals;
  }
}

async function mountWrapper(page: Page): Promise<void> {
  const wrapper = buildSurfaceFrameDocument({
    url: `${CONTENT_ORIGIN}/plugin-surfaces/first`,
    bridgeToken: "proof",
  });
  // The host arms its listener before srcdoc is assigned, as PluginSurfaceFrame does.
  await page.setContent(`<script>
    window.signals = { errors: 0, navigated: 0 };
    addEventListener("message", event => {
      if (event.source !== document.getElementById("host").contentWindow) return;
      if (event.data?.type === "multica:plugin-surface-error") window.signals.errors += 1;
      if (event.data?.type === "multica:plugin-surface-navigated") window.signals.navigated += 1;
    });
  </script><iframe id="host" sandbox="allow-scripts allow-same-origin"></iframe>`);
  await page.locator("#host").evaluate((frame, srcdoc) => {
    (frame as HTMLIFrameElement).srcdoc = srcdoc as string;
  }, wrapper);
}

function readSignals(page: Page): Promise<SurfaceSignals | null> {
  return page.evaluate(() => window.signals ?? null);
}

test.describe("plugin surface document (real Chromium, hosted frame)", () => {
  test("a guest's first-line error reaches the host once", async ({ page }) => {
    await page.route(`${CONTENT_ORIGIN}/**`, (route) =>
      route.fulfill({
        contentType: "text/html",
        headers: { "Content-Security-Policy": "default-src 'none'; script-src 'unsafe-inline'" },
        body: `<!doctype html><script>
          addEventListener("error", () => parent.postMessage({ type: "multica:plugin-surface-error" }, "*"));
        </script><script>
          throw new Error("plugin failed during bootstrap");
        </script><script>
          parent.postMessage({ type: "multica:plugin-surface-error" }, "*");
        </script>`,
      }),
    );

    await mountWrapper(page);

    await expect.poll(() => readSignals(page)).toEqual({ errors: 1, navigated: 0 });
  });

  test("the guest's own load is not a navigation, but a second document is", async ({ page }) => {
    let requests = 0;
    await page.route(`${CONTENT_ORIGIN}/**`, (route) => {
      requests += 1;
      const first = route.request().url().endsWith("/first");
      return route.fulfill({
        contentType: "text/html",
        headers: { "Content-Security-Policy": "default-src 'none'; script-src 'unsafe-inline'" },
        body: first
          ? `<!doctype html><button id="go" onclick="location.assign('/plugin-surfaces/second')">go</button>`
          : `<!doctype html><p>second</p>`,
      });
    });

    await mountWrapper(page);

    const guest = page.frameLocator("#host").frameLocator("iframe");
    await expect(guest.locator("#go")).toBeVisible();
    expect(await readSignals(page)).toEqual({ errors: 0, navigated: 0 });

    await guest.locator("#go").click();

    await expect.poll(() => readSignals(page)).toEqual({ errors: 0, navigated: 1 });
    expect(requests).toBe(2);
  });
});
