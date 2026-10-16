import { test, expect } from "@playwright/test";
import { loginAsDefault, waitForPageText } from "./helpers";

const ROUTE_CHANGE_TIMEOUT = 30000;

test.describe("Navigation", () => {
  test.beforeEach(async ({ page }) => {
    await loginAsDefault(page);
  });

  test("sidebar navigation works", async ({ page }) => {
    await page.getByRole("link", { name: "Inbox" }).click();
    await expect(page).toHaveURL(/\/inbox/, { timeout: ROUTE_CHANGE_TIMEOUT });
    await waitForPageText(page, "Inbox");
    // Each destination renames the browser tab after itself (MUL-6222).
    await expect(page).toHaveTitle("Inbox | Multica");

    await page.getByRole("link", { name: "Agents" }).click();
    await expect(page).toHaveURL(/\/agents/, { timeout: ROUTE_CHANGE_TIMEOUT });
    await waitForPageText(page, "Agents");
    await expect(page).toHaveTitle("Agents | Multica");

    await page.getByRole("link", { name: "Issues", exact: true }).click();
    await expect(page).toHaveURL(/\/issues/, { timeout: ROUTE_CHANGE_TIMEOUT });
    await waitForPageText(page, "Issues");
    await expect(page).toHaveTitle("Issues | Multica");
  });

  test("settings page loads via sidebar", async ({ page }) => {
    await page.getByRole("link", { name: "Settings", exact: true }).click();
    await expect(page).toHaveURL(/\/settings/, { timeout: ROUTE_CHANGE_TIMEOUT });
    await waitForPageText(page, "Settings");

    // Settings is a page with its own navigation: each section is a link that
    // opens its form beside it.
    const settingsNav = page.getByRole("navigation", { name: "Settings" });

    await settingsNav.getByRole("link", { name: "General" }).click();
    await expect(page).toHaveURL(/tab=workspace/);
    await expect(page.getByRole("heading", { name: "General", level: 2 })).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Name" })).toHaveValue(/E2E Workspace/);

    await settingsNav.getByRole("link", { name: "Members" }).click();
    await expect(page).toHaveURL(/tab=members/);
    await expect(page.getByRole("heading", { name: "Members", level: 2 })).toBeVisible();
  });

  test("agents page shows agent list", async ({ page }) => {
    await page.getByRole("link", { name: "Agents" }).click();
    await expect(page).toHaveURL(/\/agents/, { timeout: ROUTE_CHANGE_TIMEOUT });
    await waitForPageText(page, "Agents");

    // Should show "Agents" heading
    await expect(page.locator("text=Agents").first()).toBeVisible();
  });
});
