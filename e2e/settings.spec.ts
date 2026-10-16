import { test, expect } from "@playwright/test";
import { enableComposioApps, loginAsDefault } from "./helpers";

test.describe("Settings", () => {
  test("updating workspace name reflects in sidebar immediately", async ({
    page,
  }) => {
    const workspaceSlug = await loginAsDefault(page);

    await page.goto(`/${workspaceSlug}/settings?tab=workspace`, {
      waitUntil: "domcontentloaded",
    });
    const nameInput = page.getByRole("textbox", { name: "Name", exact: true });
    await expect(nameInput).toBeVisible();
    const originalName = await nameInput.inputValue();
    const savedState = page.getByRole("region", { name: "Workspace details" }).getByRole("status");
    const newName = "Renamed WS " + Date.now();

    try {
      await nameInput.fill(newName);
      await nameInput.blur();
      await expect(savedState).toHaveText("Saved");

      // The sidebar updates from the confirmed autosave, without a refresh.
      await expect(page.getByRole("button", { name: new RegExp(newName) })).toBeVisible();

      // A fresh document must read the persisted name, not just cached state.
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(nameInput).toHaveValue(newName);
      await expect(page.getByRole("button", { name: new RegExp(newName) })).toBeVisible();
    } finally {
      // Restore through the same user-facing autosave, including on failure.
      await nameInput.fill(originalName);
      await nameInput.blur();
      await expect(savedState).toHaveText("Saved");
      await expect(page.getByRole("button").filter({ hasText: originalName }).first()).toBeVisible();
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(nameInput).toHaveValue(originalName);
    }
  });

  // Composio connect flow, fully mocked at the network boundary so it runs
  // without a configured COMPOSIO_API_KEY or a live Composio project. The
  // backend redirect is simulated by pointing the init endpoint's redirect_url
  // straight back at the settings page with ?connected=<slug> — exercising the
  // frontend's callback toast + connections refresh (MUL-3718) end to end.
  test("connecting a Composio toolkit shows a toast and refreshes the list", async ({
    page,
  }) => {
    await enableComposioApps(page);
    const workspaceSlug = await loginAsDefault(page);
    let refreshedConnections = 0;

    // Stateful: connections is empty until the (mocked) connect flow lands.
    let connected = false;

    await page.route("**/api/integrations/composio/toolkits", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          { slug: "notion", name: "Notion", connectable: true },
        ]),
      }),
    );

    await page.route("**/api/integrations/composio/connections", (route) => {
      if (route.request().method() !== "GET") return route.fallback();
      if (connected) refreshedConnections += 1;
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          connected
            ? [
                {
                  id: "conn-notion-1",
                  toolkit_slug: "notion",
                  status: "active",
                  connected_at: new Date().toISOString(),
                  last_used_at: null,
                },
              ]
            : [],
        ),
      });
    });

    await page.route("**/api/integrations/composio/connect/init", (route) => {
      // Composio would 302 through its hosted consent and back to our callback,
      // which emits CallbackRedirect's slug-less shape:
      // `/settings?tab=integrations&connected=<slug>`. The web proxy's
      // legacy-route redirect then prepends the last workspace slug, landing on
      // the real settings route. Mock that exact backend shape (NOT the final
      // slugged URL) so the test exercises the same redirect path real users hit.
      connected = true;
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          redirect_url: `/settings?tab=integrations&connected=notion`,
        }),
      });
    });

    await page.goto(`/${workspaceSlug}/settings`, { waitUntil: "domcontentloaded" });
    const appsLink = page.getByRole("navigation", { name: "Settings", exact: true })
      .getByRole("link", { name: "Connected apps", exact: true });
    await expect(appsLink).toBeVisible();
    await appsLink.click();
    await expect(page.getByRole("heading", { name: "Connected apps", exact: true })).toBeVisible();
    await expect(appsLink).toHaveAttribute("aria-current", "page");
    await expect(page.getByText("Notion", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: /Disconnect/ })).toHaveCount(0);

    // Observe the slug-less legacy callback URL emitted by the real backend,
    // rather than substituting the final workspace-scoped Apps URL.
    const legacyCallback = page.waitForRequest((request) =>
      request.isNavigationRequest() &&
      request.url().endsWith("/settings?tab=integrations&connected=notion"),
    );
    await page.getByRole("button", { name: "Connect", exact: true }).click();
    await legacyCallback;
    await expect(
      page.locator("[data-sonner-toast]").getByText("Connected", { exact: true }),
    ).toBeVisible({ timeout: 10000 });

    // Callback consumption refreshes the list and canonicalizes the old URL.
    await expect(page.getByRole("heading", { name: "Connected apps", exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: /Disconnect/ })).toBeVisible({ timeout: 10000 });
    await expect.poll(() => refreshedConnections).toBeGreaterThan(0);
    await expect(page).toHaveURL(
      new RegExp(`/${workspaceSlug}/settings\\?tab=apps$`),
    );
  });
});
