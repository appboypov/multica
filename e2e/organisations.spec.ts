import { test, expect, type Page } from "@playwright/test";
import { createTestApi, loginAsDefault, waitForPageText } from "./helpers";
import type { TestApiClient } from "./fixtures";

/**
 * UJ-1 "Brian sees his projects by organisation", end to end: Projects groups
 * by organisation, New organisation refuses a taken name, New project starts
 * in the organisation whose page it opens from, a project's page moves it, and
 * no project status or date is shown or asked anywhere.
 *
 * The run shares its workspace with the other specs, so its organisation and
 * project names carry a suffix and the order checks look only at its own
 * groups and "No organisation".
 */

const NO_ORGANISATION = "No organisation";

async function groupNames(page: Page): Promise<string[]> {
  const toggles = page.getByRole("button", { name: /^Fold or unfold / });
  await expect(toggles.first()).toBeVisible({ timeout: 15000 });
  const labels = await toggles.evaluateAll((nodes) =>
    nodes.map((node) => node.getAttribute("aria-label") ?? ""),
  );
  return labels.map((label) => label.replace(/^Fold or unfold /, ""));
}

async function openOrganisation(page: Page, slug: string, name: string) {
  await page.goto(`/${slug}/organisations`, { waitUntil: "domcontentloaded" });
  await waitForPageText(page, name);
  await page.getByText(name, { exact: true }).first().click();
  await expect(page.getByRole("heading", { name, level: 1 })).toBeVisible({ timeout: 15000 });
}

test.describe("Projects by organisation", () => {
  let api: TestApiClient;
  const suffix = Date.now().toString(36);
  const das = `De App Specialist ${suffix}`;
  const rebl = `Rebl Records ${suffix}`;
  const multica = `Multica ${suffix}`;
  const reblProject = `Rebl ${suffix}`;
  const overview = `das-overview ${suffix}`;
  const fanApp = `Fan app ${suffix}`;

  test.beforeEach(async () => {
    api = await createTestApi();
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("Brian sees his projects by organisation", async ({ page }) => {
    const dasOrganisation = await api.createOrganisation(das, `das-${suffix}`);
    const reblOrganisation = await api.createOrganisation(rebl, `rebl-${suffix}`);
    await api.createProject(multica, { organisation_id: dasOrganisation.id });
    await api.createProject(reblProject, { organisation_id: reblOrganisation.id });
    await api.createProject(overview);

    const slug = await loginAsDefault(page);

    // Projects: De App Specialist, then Rebl Records, then No organisation last.
    await page.goto(`/${slug}/projects`, { waitUntil: "domcontentloaded" });
    await waitForPageText(page, multica);
    const groups = (await groupNames(page)).filter(
      (name) => name.endsWith(suffix) || name === NO_ORGANISATION,
    );
    expect(groups).toEqual([das, rebl, NO_ORGANISATION]);
    await expect(page.getByText(overview, { exact: true })).toBeVisible();

    // A group folds: its rows hide and its group row stays.
    await page.getByRole("button", { name: `Fold or unfold ${rebl}` }).click();
    await expect(page.getByText(reblProject, { exact: true })).toHaveCount(0);
    await expect(page.getByRole("link", { name: rebl })).toBeVisible();
    await page.getByRole("button", { name: `Fold or unfold ${rebl}` }).click();
    await expect(page.getByText(reblProject, { exact: true })).toBeVisible();

    // No project status on Projects: no Status column and no status label.
    await expect(page.getByText("Status", { exact: true })).toHaveCount(0);
    await expect(page.getByText("Planned", { exact: true })).toHaveCount(0);

    // New organisation refuses a taken name, ignoring case.
    await page.goto(`/${slug}/organisations`, { waitUntil: "domcontentloaded" });
    await waitForPageText(page, rebl);
    await page.getByRole("button", { name: "New organisation" }).first().click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Name").fill(rebl.toLowerCase());
    await dialog.getByLabel("Key").fill(`fresh-${suffix}`);
    await expect(dialog.getByText("Another organisation has this name")).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Create" })).toBeDisabled();
    await dialog.getByRole("button", { name: "Cancel" }).click();

    // New project from Rebl Records' page starts in Rebl Records.
    await openOrganisation(page, slug, rebl);
    await expect(page.getByText("1 project", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "New project" }).first().click();
    const create = page.getByRole("dialog");
    await expect(create.getByText(rebl, { exact: true })).toBeVisible();
    // No status or date is asked.
    await expect(create.getByText("Planned", { exact: true })).toHaveCount(0);
    await expect(create.getByText(/start date|due date/i)).toHaveCount(0);
    await create.getByRole("textbox", { name: /project title/i }).fill(fanApp);
    await create.getByRole("button", { name: /^create project$/i }).click();

    // Create opens the new project's page, which names its organisation and
    // shows no status or date.
    await expect(page.getByRole("button", { name: rebl })).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(/^(Status|Start date|Due date)$/)).toHaveCount(0);

    await openOrganisation(page, slug, rebl);
    await expect(page.getByText(fanApp, { exact: true })).toBeVisible();
    await expect(page.getByText("2 projects", { exact: true })).toBeVisible();

    // Moving Fan app to De App Specialist on its page moves it on both pages.
    await page.getByText(fanApp, { exact: true }).click();
    await page.getByRole("button", { name: rebl }).click();
    await page.getByRole("menuitem", { name: das }).click();
    await expect(page.getByRole("button", { name: das })).toBeVisible({ timeout: 10000 });

    await openOrganisation(page, slug, das);
    await expect(page.getByText(fanApp, { exact: true })).toBeVisible();
    await expect(page.getByText("2 projects", { exact: true })).toBeVisible();

    await openOrganisation(page, slug, rebl);
    await expect(page.getByText(reblProject, { exact: true })).toBeVisible();
    await expect(page.getByText(fanApp, { exact: true })).toHaveCount(0);
    await expect(page.getByText("1 project", { exact: true })).toBeVisible();
  });
});
