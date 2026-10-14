import { test, expect } from "@playwright/test";
import pg from "pg";
import { loginAsDefault, createTestApi, preferManualCreateMode, reloadAppPage } from "./helpers";
import type { TestApiClient } from "./fixtures";

const DATABASE_URL =
  process.env.DATABASE_URL ?? "postgres://multica:multica@localhost:5432/multica?sslmode=disable";

async function setIssueTimestamps(
  issueId: string,
  timestamps: { createdAt: Date; updatedAt?: Date },
) {
  const client = new pg.Client(DATABASE_URL);
  await client.connect();
  try {
    await client.query(
      `
        UPDATE issue
        SET created_at = $2, updated_at = $3
        WHERE id = $1
      `,
      [
        issueId,
        timestamps.createdAt.toISOString(),
        (timestamps.updatedAt ?? timestamps.createdAt).toISOString(),
      ],
    );
  } finally {
    await client.end();
  }
}

test.describe("Issues", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await loginAsDefault(page);
  });

  test.afterEach(async () => {
    if (api) {
      await api.cleanup();
    }
  });

  test("issues page loads with board view", async ({ page }) => {
    await api.createIssue("E2E Board View " + Date.now());
    await reloadAppPage(page);

    // Board columns should be visible
    await expect(page.locator("text=Backlog")).toBeVisible();
    await expect(page.locator("text=Todo")).toBeVisible();
    await expect(page.locator("text=In Progress")).toBeVisible();
  });

  test("can switch from board to list view", async ({ page }) => {
    const title = "E2E List Switch " + Date.now();
    await api.createIssue(title);
    await reloadAppPage(page);
    await expect(page.locator("text=Backlog")).toBeVisible();

    // Switch to list view
    await page.click("text=List");
    await expect(page.getByText(title)).toBeVisible();
  });

  test("can filter issues by created and updated dates", async ({ page }) => {
    const suffix = Date.now();
    const todayTitle = `E2E Date Today ${suffix}`;
    const oldTitle = `E2E Date Old ${suffix}`;
    const updatedTodayTitle = `E2E Date Updated Today ${suffix}`;
    await api.createIssue(todayTitle);
    const oldIssue = await api.createIssue(oldTitle);
    const updatedTodayIssue = await api.createIssue(updatedTodayTitle);
    const oldDate = new Date();
    oldDate.setDate(oldDate.getDate() - 8);
    await setIssueTimestamps(oldIssue.id, { createdAt: oldDate });
    await setIssueTimestamps(updatedTodayIssue.id, {
      createdAt: oldDate,
      updatedAt: new Date(),
    });

    await reloadAppPage(page);
    await expect(page.getByText(todayTitle)).toBeVisible();
    await expect(page.getByText(oldTitle)).toBeVisible();
    await expect(page.getByText(updatedTodayTitle)).toBeVisible();

    await page.getByRole("button", { name: /filter/i }).click();
    await page.getByRole("menuitem", { name: /^Date\b/ }).hover();
    await page.getByRole("menuitem", { name: "Today" }).click();

    await expect(page.getByRole("button", { name: /1 filter/i })).toBeVisible();
    await expect(page.getByText(todayTitle)).toBeVisible();
    await expect(page.getByText(oldTitle)).toBeHidden({ timeout: 10000 });
    await expect(page.getByText(updatedTodayTitle)).toBeHidden({ timeout: 10000 });

    await page.getByRole("button", { name: /1 filter/i }).click();
    const dateFilterItem = page.getByRole("menuitem", { name: /^Date\b/ });
    await dateFilterItem.focus();
    await page.keyboard.press("ArrowRight");
    const updatedDateField = page.getByRole("menuitemradio", { name: "Updated" });
    await expect(updatedDateField).toBeVisible();
    await updatedDateField.press("Enter");
    await expect(page.getByText(todayTitle)).toBeVisible();
    await expect(page.getByText(updatedTodayTitle)).toBeVisible();
    await expect(page.getByText(oldTitle)).toBeHidden({ timeout: 10000 });
  });

  test("can filter issues by custom created date", async ({ page }) => {
    const suffix = Date.now();
    const todayTitle = `E2E Date Custom Today ${suffix}`;
    const oldTitle = `E2E Date Custom Old ${suffix}`;
    await api.createIssue(todayTitle);
    const oldIssue = await api.createIssue(oldTitle);
    const oldDate = new Date();
    oldDate.setDate(oldDate.getDate() - 8);
    await setIssueTimestamps(oldIssue.id, { createdAt: oldDate });

    await reloadAppPage(page);
    await expect(page.getByText(todayTitle)).toBeVisible();
    await expect(page.getByText(oldTitle)).toBeVisible();

    await page.getByRole("button", { name: /filter/i }).click();
    await page.getByRole("menuitem", { name: /^Date\b/ }).hover();
    const customDateButton = page.getByRole("button", { name: "Custom date or range" });
    await expect(customDateButton).toBeVisible();
    await customDateButton.click();
    const todayDataDay = await page.evaluate(() => new Date().toLocaleDateString());
    await page.locator(`[data-day="${todayDataDay}"]`).click();
    await page.getByRole("button", { name: "Apply" }).click();
    await expect(page.getByText(todayTitle)).toBeVisible();
    await expect(page.getByText(oldTitle)).toBeHidden({ timeout: 10000 });
  });

  test("can create a new issue", async ({ page }) => {
    await preferManualCreateMode(page);

    const newIssueButton = page.getByRole("button", { name: "New Issue" });
    await expect(newIssueButton).toBeVisible();
    await newIssueButton.click();

    const title = "E2E Created " + Date.now();
    const titleInput = page.getByRole("textbox", { name: "Issue title" });
    await expect(titleInput).toBeVisible();
    await titleInput.fill(title);
    await page.getByRole("button", { name: "Create Issue" }).click();

    await expect(page.getByText("Issue created")).toBeVisible({ timeout: 10000 });
    await expect(
      page.getByRole("region", { name: /Notifications/ }).getByText(title),
    ).toBeVisible();

    await page.getByRole("button", { name: "View issue" }).click();
    await page.waitForURL(/\/issues\/[\w-]+/);
    await expect(page.locator("text=Properties")).toBeVisible();
  });

  test("New issue starts from No template or one of eight templates / New issue opens", async ({ page }) => {
    await preferManualCreateMode(page);
    await page.getByRole("button", { name: "New Issue", exact: true }).click();

    const dialog = page.getByRole("dialog", { name: "New Issue", exact: true });
    const templates = dialog.getByRole("radiogroup", { name: "Template", exact: true });
    const names = [
      "No template",
      "Epic",
      "Journey",
      "Story",
      "Bug",
      "Enhancement",
      "Refactor",
      "Chore",
      "Spike",
    ];
    await expect(templates.getByRole("radio")).toHaveText(
      names.map((name) => new RegExp(`^${name}`)),
    );
    for (const name of names) {
      await expect(templates.getByRole("radio", { name: new RegExp(`\\b${name}\\b`) })).toBeVisible();
    }
    await expect(templates.getByRole("radio", { name: /\bNo template\b/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    await expect(templates.getByRole("radio", { checked: true })).toHaveCount(1);
    await expect(dialog.getByText("No type", { exact: true })).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Create Issue", exact: true })).toBeVisible();
  });

  test("New issue starts from No template or one of eight templates / Creating an issue with No template", async ({ page }) => {
    await preferManualCreateMode(page);
    await page.getByRole("button", { name: "New Issue", exact: true }).click();

    const dialog = page.getByRole("dialog", { name: "New Issue", exact: true });
    await expect(dialog.getByRole("radio", { name: /\bNo template\b/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    const title = `Check the guest list export ${Date.now()}`;
    const description = "It misses the last row";
    await dialog.getByRole("textbox", { name: "Issue title" }).fill(title);
    await dialog.locator('.rich-text-editor[contenteditable="true"]').fill(description);

    const createdResponse = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/issues" &&
        response.request().method() === "POST" &&
        response.request().postDataJSON()?.title === title,
    );
    await dialog.getByRole("button", { name: "Create Issue", exact: true }).click();
    const response = await createdResponse;
    expect(response.ok()).toBe(true);
    const created = await response.json();
    try {
      await expect(page.getByText("Issue created", { exact: true })).toBeVisible();
      const issue = await api.getIssue(created.id);
      expect(issue).toHaveProperty("type", null);
      expect(issue).toHaveProperty("description", expect.stringContaining(description));
    } finally {
      await api.deleteIssue(created.id);
    }
  });

  test("Create stays disabled until the title is filled / Only the title filled", async ({ page }) => {
    await preferManualCreateMode(page);
    await page.getByRole("button", { name: "New Issue", exact: true }).click();

    const dialog = page.getByRole("dialog", { name: "New Issue", exact: true });
    await dialog.getByRole("radio", { name: /\bChore\b/ }).click();
    const create = dialog.getByRole("button", { name: "Create chore", exact: true });
    await expect(create).toHaveAttribute("aria-disabled", "true");

    const title = `Renew the TLS certificate ${Date.now()}`;
    await dialog.getByRole("textbox", { name: "Issue title" }).fill(title);
    await expect(create).not.toHaveAttribute("aria-disabled");

    const createdResponse = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/issues" &&
        response.request().method() === "POST" &&
        response.request().postDataJSON()?.title === title,
    );
    await create.click();
    const response = await createdResponse;
    expect(response.ok()).toBe(true);
    const created = await response.json();
    try {
      const issue = await api.getIssue(created.id);
      expect(issue).toHaveProperty("type", "chore");
      expect(issue).toHaveProperty("description", expect.stringContaining("Acceptance criteria"));
    } finally {
      await api.deleteIssue(created.id);
    }
  });

  test("can navigate to issue detail page", async ({ page }) => {
    // Create a known issue via API so the test controls its own fixture
    const issue = await api.createIssue("E2E Detail Test " + Date.now());

    // Reload to see the new issue
    await reloadAppPage(page);

    // Navigate to the issue detail. Use a suffix match so the selector works
    // whether the href is legacy `/issues/{id}` or URL-refactored
    // `/{slug}/issues/{id}`.
    const issueLink = page.locator(`a[href$="/issues/${issue.id}"]`);
    await expect(issueLink).toBeVisible({ timeout: 5000 });
    await issueLink.click();

    await page.waitForURL(/\/issues\/[\w-]+/);

    // Should show Properties panel
    await expect(page.locator("text=Properties")).toBeVisible();
    // Should show breadcrumb link back to Issues
    await expect(
      page.locator("a", { hasText: "Issues" }).first(),
    ).toBeVisible();
    // The browser tab must name the issue, so several open at once stay
    // distinguishable without clicking into each (MUL-6222).
    await expect(page).toHaveTitle(
      `${issue.identifier}: ${issue.title} | Multica`,
    );
  });

  test("can dismiss issue creation", async ({ page }) => {
    await preferManualCreateMode(page);

    await page.getByRole("button", { name: "New Issue" }).click();

    const titleInput = page.getByRole("textbox", { name: "Issue title" });
    await expect(titleInput).toBeVisible();

    await page.keyboard.press("Escape");

    await expect(titleInput).not.toBeVisible();
    await expect(page.getByRole("button", { name: "New Issue" })).toBeVisible();
  });
});
