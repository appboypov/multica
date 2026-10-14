import { test, expect, type Locator, type Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { createTestApi, loginAsDefault, waitForPageText } from "./helpers";
import { githubRepository, startGitHubFake, stopGitHubFake, type FakeGitHub } from "./github-fake";
import type {
  TestApiClient, TestRepository, TestRepositoryCheckout, TestRepositoryCheckoutResult,
  TestWorklaneAnswer, TestWorklaneStep,
} from "./fixtures";

const REBL = "appboypov/rebl";
const ADMIN = "appboypov/rebl-admin";
const ACTOR = "E2E User";
const WORKTREES = ["worklane-worktree-v1"];
const PROFILE = "Fans make an account, sign in and keep their own profile with a photo.";
const MEMBERSHIPS = "Fixes for invitations and memberships found after the first release.";

/** The runtime expands `~` to its user's home, as the real daemon does. */
function resolved(path: string): string {
  return path.startsWith("~/") ? `/home/e2e/${path.slice(2)}` : path;
}

/** appboypov/rebl has no tag yet, and appboypov/rebl-admin has v0.4.2. */
function answerCheckout(request: TestRepositoryCheckout): TestRepositoryCheckoutResult {
  return {
    status: "completed",
    resolved_path: resolved(request.path),
    passed: true,
    failures: [],
    version_files: ["package.json"],
    last_tag: request.full_name === ADMIN ? "v0.4.2" : "",
    install_commands: [],
  };
}

/** The daemon side of a worklane creation: the worktree is made at once, with nothing to install. */
const answerWorklane: TestWorklaneAnswer = async (request, report) => {
  if (request.kind !== "create") throw new Error(`Unexpected worklane request: ${request.kind}`);
  const steps: TestWorklaneStep[] = (["fetch", "add_worktree", "install", "open"] as const).map((key) => ({
    key,
    state: key === "open" ? "pending" : key === "install" ? "skipped" : "done",
    detail: "",
    output: "",
    duration_ms: 0,
  }));
  await report({ steps, path: resolved(request.path) });
};

function cardOf(page: Page, fullName: string) {
  const toggle = page.getByRole("button", { name: `Fold or unfold ${fullName}`, exact: true });
  return { toggle, card: page.locator("section").filter({ has: toggle }) };
}

async function openCard(page: Page, fullName: string): Promise<Locator> {
  const { toggle, card } = cardOf(page, fullName);
  await expect(toggle).toBeVisible({ timeout: 15000 });
  if ((await toggle.getAttribute("aria-expanded")) !== "true") await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  return card;
}

function worklaneRow(card: Locator, name: string): Locator {
  return card.getByRole("listitem").filter({ has: card.page().getByRole("link", { name: new RegExp(`^${name}\\s*Free`) }) });
}

/**
 * Picks an option of an open-able select from the keyboard: the popup scrolls
 * its list under the pointer, so a pointer click never finds the option stable.
 */
async function chooseOption(page: Page, trigger: Locator, name: string) {
  await trigger.click();
  const option = page.getByRole("option", { name, exact: true });
  await expect(option).toBeAttached();
  // The list opens on the selected option, which may sit below the wanted one.
  for (const key of ["ArrowDown", "ArrowUp"]) {
    for (let i = 0; i < 10 && (await option.getAttribute("data-highlighted")) === null; i++) {
      await page.keyboard.press(key);
    }
  }
  await expect(option).toHaveAttribute("data-highlighted", "");
  await page.keyboard.press("Enter");
  await expect(option).toHaveCount(0);
}

async function openEditRepository(page: Page, projectUrl: string, fullName: string): Promise<Locator> {
  await page.goto(projectUrl, { waitUntil: "domcontentloaded" });
  const card = await openCard(page, fullName);
  await card.getByRole("button", { name: `Actions of ${fullName}`, exact: true }).click();
  await page.getByRole("menuitem", { name: "Edit repository", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Edit repository", exact: true });
  await expect(dialog).toBeVisible();
  return dialog;
}

async function openWorklane(page: Page, slug: string, id: string, name: string) {
  await page.goto(`/${slug}/worklanes/${id}`, { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("heading", { name, level: 1 })).toBeVisible({ timeout: 15000 });
}

function timeline(page: Page): Locator {
  return page.getByRole("region", { name: "Timeline", exact: true });
}

/** A release column on the Timeline, named "{version} {title}" with the version without `v`. */
function column(page: Page, name: string): Locator {
  return timeline(page).getByRole("region", { name, exact: true });
}

async function expectLine(page: Page, names: string[]) {
  const columns = timeline(page).getByRole("region", { name: /^\d+\.\d+\.\d+ / });
  await expect
    .poll(() => columns.evaluateAll((nodes) => nodes.map((node) => node.getAttribute("aria-label"))), {
      timeout: 15000,
    })
    .toEqual(names);
}

async function openPlan(page: Page, fullName = REBL): Promise<Locator> {
  await timeline(page).getByRole("button", { name: "Plan release", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: `Plan release · ${fullName}`, exact: true });
  await expect(dialog).toBeVisible();
  return dialog;
}

function choice(dialog: Locator, name: "Patch" | "Minor" | "Major" | "Custom"): Locator {
  return dialog.getByRole("radio", { name: new RegExp(`^${name}`) });
}

async function expectOptions(dialog: Locator, patch: string, minor: string, major: string) {
  await expect(choice(dialog, "Patch")).toContainText(patch);
  await expect(choice(dialog, "Minor")).toContainText(minor);
  await expect(choice(dialog, "Major")).toContainText(major);
  await expect(choice(dialog, "Custom")).toBeVisible();
}

/** The open release menu. A menu closed a moment earlier stays in the DOM while it fades. */
function openedMenu(page: Page): Locator {
  return page.locator('[role="menu"][data-open]');
}

async function openMenu(page: Page, release: string) {
  await column(page, release).getByRole("button", { name: new RegExp(`^Actions of v${release}$`) }).click();
  await expect(openedMenu(page)).toBeVisible();
}

function menuItem(page: Page, label: string): Locator {
  return openedMenu(page).getByRole("menuitem", { name: new RegExp(`^${label}`) });
}

async function openEdit(page: Page, release: string): Promise<Locator> {
  await openMenu(page, release);
  await menuItem(page, "Edit release").click();
  const dialog = page.getByRole("dialog", { name: `Edit release · ${REBL}`, exact: true });
  await expect(dialog).toBeVisible();
  return dialog;
}

/** The worklane's activity entries, without the changed versions listed inside them. */
function entries(page: Page): Locator {
  return page.getByRole("region", { name: "Activity", exact: true }).locator("ol > li");
}

function entry(page: Page, sentence: string): Locator {
  return entries(page).filter({ hasText: sentence });
}

async function expectEntry(page: Page, sentence: string, changes: string[]) {
  const found = entry(page, sentence);
  await expect(found).toHaveCount(1, { timeout: 15000 });
  await expect(found.locator("time")).toHaveCount(1);
  await expect(found.locator("ul > li")).toHaveText(changes);
}

test.describe("Release line of a repository / UJ-4", () => {
  let github: FakeGitHub;
  let api: TestApiClient | undefined;
  let suffix: string;

  test.beforeAll(async () => {
    github = await startGitHubFake();
  });
  test.afterAll(async () => {
    await stopGitHubFake(github);
  });
  test.beforeEach(async () => {
    suffix = `${Date.now().toString(36)}-${randomUUID().slice(0, 8)}`;
    github.repositories.splice(
      0,
      github.repositories.length,
      ...[REBL, ADMIN].map((name, index) => githubRepository(name, 930001 + index)),
    );
    api = await createTestApi();
    await api.seedGitHubInstallation(Date.now());
  });
  test.afterEach(async () => {
    await api?.cleanup();
  });

  /**
   * Rebl Records (organisation Rebl) on Brian's Mac, a runtime that checks
   * checkouts and makes worklanes, with one repository checked out on it.
   */
  async function setupProject(page: Page, client: TestApiClient, fullName: string, checkout: string) {
    const organisation = await client.createOrganisation(`Rebl ${suffix}`, "rebl");
    const projectName = `Rebl Records ${suffix}`;
    const project = await client.createProject(projectName, { organisation_id: organisation.id });
    const runtimeName = `Brian's Mac ${suffix}`;
    const runtime = await client.registerRepositoryRuntime(runtimeName, "online", WORKTREES);
    await client.startRepositoryCheckoutAnswerer(runtime.id, answerCheckout, undefined, answerWorklane);
    const check = await client.checkRepositoryCheckout(runtime.id, fullName, checkout);
    const repository: TestRepository = await client.createRepository(project.id, fullName, check, "");
    const slug = await loginAsDefault(page);
    const projectUrl = `/${slug}/projects/${project.id}`;
    return { organisation, project, projectName, runtime, runtimeName, repository, slug, projectUrl };
  }

  /** appboypov/rebl with the worklanes `main`, Fan accounts and push, and Agenda and publication. */
  async function setupRebl(page: Page, client: TestApiClient) {
    const context = await setupProject(page, client, REBL, "/home/e2e/Repos/rebl");
    const daemon = context.runtime.daemon_id;
    const fan = await client.createWorklane(context.repository.id, "Fan accounts and push", "fan-accounts-and-push", daemon);
    const agenda = await client.createWorklane(context.repository.id, "Agenda and publication", "agenda-and-publication", daemon);
    const main = (await client.listRepositoryWorklanes(context.repository.id)).find((worklane) => worklane.is_main);
    if (!main) throw new Error("appboypov/rebl has no main worklane");
    return { ...context, fan, agenda, main: main.id };
  }

  test("Brian plans, moves and edits appboypov/rebl's releases on its three worklanes", async ({ page }) => {
    test.setTimeout(150000);
    if (!api) throw new Error("Test API setup did not finish");
    const client = api;
    const context = await setupRebl(page, client);
    const { slug, fan, agenda, main } = context;

    await openWorklane(page, slug, fan, "Fan accounts and push");
    await expect(timeline(page).getByText("No releases yet", { exact: true })).toBeVisible();

    // plan-release: The first release without a tag.
    let dialog = await openPlan(page);
    await expectOptions(dialog, "v0.0.1", "v0.1.0", "v1.0.0");
    await expect(choice(dialog, "Patch")).toHaveAttribute("aria-checked", "true");
    await expect(dialog.getByText("The first release of this repository.", { exact: true })).toBeVisible();
    // plan-release: No title.
    await expect(dialog.getByRole("button", { name: "Plan v0.0.1", exact: true })).toBeDisabled();
    // plan-release: Patch picked, the Ships line without a pass.
    await expect(
      dialog.getByText(
        "When its issues are done and merged, Multica tags v0.0.1 on main, writes the release note and publishes the GitHub release. It gets its date when it ships.",
        { exact: true },
      ),
    ).toBeVisible();
    await expect(dialog).not.toContainText(/\bpass\b/i);

    // plan-release: The first release becomes next.
    await choice(dialog, "Major").click();
    await dialog.getByLabel("Title", { exact: true }).fill("First production release");
    await dialog.getByRole("button", { name: "Plan v1.0.0", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(column(page, "1.0.0 First production release").getByText("Next release", { exact: true })).toBeVisible();

    dialog = await openPlan(page);
    await expect(
      dialog.getByText("Follows v1.0.0 First production release. Moving it renumbers it.", { exact: true }),
    ).toBeVisible();
    await dialog.getByLabel("Title", { exact: true }).fill("Membership hardening");
    await dialog.getByRole("button", { name: "Plan v1.0.1", exact: true }).click();
    await expect(dialog).toHaveCount(0);

    dialog = await openPlan(page);
    // plan-release: Not a version.
    await choice(dialog, "Custom").click();
    const custom = dialog.getByLabel("Custom version");
    await custom.fill("1.4");
    await expect(dialog.getByText("Use the form v1.4.0.", { exact: true })).toBeVisible();
    await expect(dialog.getByRole("button", { name: /^Plan/ })).toBeDisabled();
    // plan-release: Too low after the last release.
    await dialog.getByLabel("Title", { exact: true }).fill("Account and profile");
    await custom.fill("v1.0.0");
    await expect(
      dialog.getByText("Must be higher than v1.0.1 Membership hardening, the release before it.", { exact: true }),
    ).toBeVisible();
    await expect(custom).toHaveAttribute("aria-invalid", "true");
    await expect(dialog.getByRole("button", { name: "Plan v1.0.0", exact: true })).toBeDisabled();
    // plan-release: Written without v.
    await custom.fill("1.4.0");
    await expect(dialog.getByRole("button", { name: "Plan v1.4.0", exact: true })).toBeEnabled();
    await choice(dialog, "Minor").click();
    await dialog.getByLabel("Description", { exact: true }).fill(PROFILE);
    await dialog.getByRole("button", { name: "Plan v1.1.0", exact: true }).click();
    await expect(dialog).toHaveCount(0);

    // plan-release: After the last planned release.
    dialog = await openPlan(page);
    await expectOptions(dialog, "v1.1.1", "v1.2.0", "v2.0.0");
    await expect(choice(dialog, "Patch")).toHaveAttribute("aria-checked", "true");
    await expect(
      dialog.getByText("Follows v1.1.0 Account and profile. Moving it renumbers it.", { exact: true }),
    ).toBeVisible();
    // plan-release: Title without description.
    await dialog.getByLabel("Title", { exact: true }).fill("Notifications");
    await choice(dialog, "Minor").click();
    await expect(dialog.getByLabel(/goal|date/i)).toHaveCount(0);
    // plan-release: Notifications planned.
    await dialog.getByRole("button", { name: "Plan v1.2.0", exact: true }).click();
    await expect(dialog).toHaveCount(0);

    // worklane-release-line: Three releases on appboypov/rebl, then the fourth.
    const planned = [
      "1.0.0 First production release",
      "1.0.1 Membership hardening",
      "1.1.0 Account and profile",
      "1.2.0 Notifications",
    ];
    await expectLine(page, planned);
    await expect(column(page, planned[0]).getByText("Next release", { exact: true })).toBeVisible();
    for (const name of planned) {
      if (name !== planned[0]) await expect(column(page, name).getByText("Planned", { exact: true })).toBeVisible();
      await expect(column(page, name).getByText("No issues of this worklane", { exact: true })).toBeVisible();
      // worklane-release-line: Planned releases on Fan accounts and push.
      await expect(column(page, name)).not.toContainText(/goal|20\d\d/i);
    }
    await expect(timeline(page).getByRole("button", { name: "Plan release", exact: true })).toBeVisible();
    // worklane-release-line: A release with a description.
    await expect(column(page, planned[2]).getByText(PROFILE, { exact: true })).toBeVisible();
    await expect(page.getByText("nothing shipped yet", { exact: true })).toBeVisible();

    // edit-release: The menu of v1.1.0.
    await openMenu(page, planned[2]);
    for (const item of ["Edit release", "Move earlier", "Move later", "Remove release"]) {
      await expect(menuItem(page, item)).toBeVisible();
    }
    // edit-release: Filled in.
    await menuItem(page, "Edit release").click();
    dialog = page.getByRole("dialog", { name: `Edit release · ${REBL}`, exact: true });
    await expect(choice(dialog, "Minor")).toHaveAttribute("aria-checked", "true");
    await expectOptions(dialog, "v1.0.2", "v1.1.0", "v2.0.0");
    const title = dialog.getByLabel("Title", { exact: true });
    await expect(title).toHaveValue("Account and profile");
    await expect(dialog.getByLabel("Description", { exact: true })).toHaveValue(PROFILE);
    // edit-release: No title.
    await title.fill("");
    await expect(dialog.getByRole("button", { name: "Save release", exact: true })).toBeDisabled();
    await title.fill("Account and profile");
    // edit-release: Too low.
    await choice(dialog, "Custom").click();
    const editCustom = dialog.getByLabel("Custom version");
    await editCustom.fill("v1.0.1");
    await expect(
      dialog.getByText("Must be higher than v1.0.1 Membership hardening, the release before it.", { exact: true }),
    ).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Save release", exact: true })).toBeDisabled();
    // edit-release: Account and profile set to v1.2.5, what renumbers.
    await editCustom.fill("v1.2.5");
    await expect(dialog.getByText("Later releases renumber: v1.2.0 → v1.3.0.", { exact: true })).toBeVisible();
    await expect(
      dialog.getByText(
        "Set by hand: it keeps this number when the release moves, and must stay higher than the release before it.",
        { exact: true },
      ),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expectLine(page, planned);

    // move-release: The first release.
    await openMenu(page, planned[0]);
    await expect(menuItem(page, "Move earlier")).toBeDisabled();
    await expect(menuItem(page, "Move earlier")).toContainText("It is the first release");
    await page.keyboard.press("Escape");
    // move-release: The last release.
    await openMenu(page, planned[3]);
    await expect(menuItem(page, "Move later")).toBeDisabled();
    await expect(menuItem(page, "Move later")).toContainText("It is the last release");
    // move-release: Notifications moved earlier.
    await menuItem(page, "Move earlier").click();
    const moved = [
      "1.0.0 First production release",
      "1.0.1 Membership hardening",
      "1.1.0 Notifications",
      "1.2.0 Account and profile",
    ];
    await expectLine(page, moved);

    // Account and profile set by hand to v1.2.5, the last on the line.
    dialog = await openEdit(page, moved[3]);
    await choice(dialog, "Custom").click();
    await dialog.getByLabel("Custom version").fill("v1.2.5");
    await expect(dialog.getByText(/^Later releases renumber/)).toHaveCount(0);
    await dialog.getByRole("button", { name: "Save release", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    const signal = [
      "1.0.0 First production release",
      "1.0.1 Membership hardening",
      "1.1.0 Notifications",
      "1.2.5 Account and profile",
    ];
    await expectLine(page, signal);
    await expect(column(page, signal[3]).getByText("Set by hand", { exact: true })).toBeVisible();

    // release-line-activity: Edited without a version change.
    dialog = await openEdit(page, signal[1]);
    await dialog.getByLabel("Description", { exact: true }).fill(MEMBERSHIPS);
    await dialog.getByRole("button", { name: "Save release", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(column(page, signal[1]).getByText(MEMBERSHIPS, { exact: true })).toBeVisible();

    // worklane-release-line: The same line on every worklane, and each worklane's activity.
    const lanes: [string, string][] = [[fan, "Fan accounts and push"], [main, "main"], [agenda, "Agenda and publication"]];
    for (const [id, name] of lanes) {
      await openWorklane(page, slug, id, name);
      await expectLine(page, signal);
      await expect(column(page, signal[3]).getByText("Set by hand", { exact: true })).toBeVisible();
      await expect(column(page, signal[3]).getByText(PROFILE, { exact: true })).toBeVisible();
      // release-line-activity: Planned.
      await expectEntry(page, `${ACTOR} planned v1.0.0 First production release`, []);
      await expectEntry(page, `${ACTOR} planned v1.0.1 Membership hardening`, []);
      await expectEntry(page, `${ACTOR} planned v1.1.0 Account and profile`, []);
      await expectEntry(page, `${ACTOR} planned v1.2.0 Notifications`, []);
      // release-line-activity: Moved with renumbering.
      await expectEntry(page, `${ACTOR} moved v1.1.0 Notifications earlier`, [
        "Notifications v1.2.0 → v1.1.0",
        "Account and profile v1.1.0 → v1.2.0",
      ]);
      // release-line-activity: Edited to a version set by hand.
      await expectEntry(page, `${ACTOR} edited v1.2.5 Account and profile`, ["Account and profile v1.2.0 → v1.2.5"]);
      // release-line-activity: Edited without a version change.
      await expectEntry(page, `${ACTOR} edited v1.0.1 Membership hardening`, []);
    }

    // worklane-release-chips: appboypov/rebl with releases, on Rebl Records' page.
    await page.goto(context.projectUrl, { waitUntil: "domcontentloaded" });
    const card = await openCard(page, REBL);
    await expect(card.getByText("nothing shipped", { exact: true })).toHaveCount(3);
    await expect(card.getByRole("link", { name: /^Open v1\.0\.0 on the Timeline of / })).toHaveCount(3);
    // worklane-release-chips: Opening the Timeline from a release.
    await worklaneRow(card, "Fan accounts and push")
      .getByRole("link", { name: "Open v1.0.0 on the Timeline of Fan accounts and push", exact: true })
      .click();
    await expect(page.getByRole("heading", { name: "Fan accounts and push", level: 1 })).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/worklanes/${fan}\\?release=`));
    await expectLine(page, signal);

    // worklane-release-chips: On Projects.
    await page.goto(`/${slug}/projects`, { waitUntil: "domcontentloaded" });
    const projectFold = page.getByRole("button", {
      name: `Fold or unfold the repositories of ${context.projectName}`,
      exact: true,
    });
    await expect(projectFold).toBeVisible({ timeout: 15000 });
    if ((await projectFold.getAttribute("aria-expanded")) !== "true") await projectFold.click();
    const projectsFold = page.getByRole("button", { name: new RegExp(`^(Show|Hide) the worklanes of ${REBL}$`) });
    if ((await projectsFold.getAttribute("aria-expanded")) !== "true") await projectsFold.click();
    await expect(page.getByRole("columnheader", { name: "Last · Next · Planned releases" }).first()).toBeVisible();
    await expect(page.getByRole("link", { name: /^Open v1\.0\.0 on the Timeline of / })).toHaveCount(3);
    await expect(page.getByText("nothing shipped", { exact: true })).toHaveCount(3);

    // worklane-release-chips: appboypov/rebl with releases, on Rebl's page.
    await page.goto(`/${slug}/organisations/${context.organisation.id}`, { waitUntil: "domcontentloaded" });
    await waitForPageText(page, context.projectName);
    const organisationFold = page.getByRole("button", {
      name: `Fold or unfold the repositories of ${context.projectName}`,
      exact: true,
    });
    if ((await organisationFold.getAttribute("aria-expanded")) !== "true") await organisationFold.click();
    const repositoryFold = page.getByRole("button", { name: new RegExp(`^(Show|Hide) the worklanes of ${REBL}$`) });
    if ((await repositoryFold.getAttribute("aria-expanded")) !== "true") await repositoryFold.click();
    await expect(page.getByRole("columnheader", { name: "Last · Next · Planned releases" }).first()).toBeVisible();
    await expect(page.getByRole("link", { name: /^Open v1\.0\.0 on the Timeline of / })).toHaveCount(3);
    await expect(page.getByText("nothing shipped", { exact: true })).toHaveCount(3);

    // release-line-activity: A worklane created later.
    const fanApp = await client.createWorklane(context.repository.id, "Fan app", "fan-app", context.runtime.daemon_id);
    await openWorklane(page, slug, fanApp, "Fan app");
    await expect(entries(page)).toHaveCount(1, { timeout: 15000 });
    await expect(entries(page)).toContainText(`${ACTOR} created Fan app on ${context.runtimeName}`);
    await expectLine(page, signal);

    // remove-release: The first release removed, and a version set by hand stays.
    await openMenu(page, signal[0]);
    await menuItem(page, "Remove release").click();
    const remove = page.getByRole("alertdialog", { name: "Remove v1.0.0 First production release?", exact: true });
    await expect(remove.getByText("Later releases renumber: v1.0.1 → v0.0.1, v1.1.0 → v0.1.0.", { exact: true })).toBeVisible();
    await remove.getByRole("button", { name: "Remove release", exact: true }).click();
    await expect(remove).toHaveCount(0);
    const removed = ["0.0.1 Membership hardening", "0.1.0 Notifications", "1.2.5 Account and profile"];
    for (const [id, name] of lanes) {
      await openWorklane(page, slug, id, name);
      await expectLine(page, removed);
      await expect(column(page, removed[0]).getByText("Next release", { exact: true })).toBeVisible();
      await expect(column(page, removed[2]).getByText("Set by hand", { exact: true })).toBeVisible();
      // release-line-activity: Removed.
      await expectEntry(page, `${ACTOR} removed v1.0.0 First production release`, [
        "Membership hardening v1.0.1 → v0.0.1",
        "Notifications v1.1.0 → v0.1.0",
      ]);
    }
  });

  test("Refused moves and edits, the last release removed and appboypov/rebl stopping releasing", async ({ page }) => {
    test.setTimeout(120000);
    if (!api) throw new Error("Test API setup did not finish");
    const client = api;
    const context = await setupRebl(page, client);
    const { slug, fan, main, agenda } = context;
    const rebl = context.repository.id;
    await client.planRelease(rebl, { title: "First production release", bump: "major" });
    await client.planRelease(rebl, { title: "Membership hardening", version: "1.0.1" });
    await client.planRelease(rebl, { title: "Account and profile", bump: "minor" });
    await client.planRelease(rebl, { title: "Notifications", bump: "minor" });
    const seeded = [
      "1.0.0 First production release",
      "1.0.1 Membership hardening",
      "1.1.0 Account and profile",
      "1.2.0 Notifications",
    ];

    await openWorklane(page, slug, fan, "Fan accounts and push");
    await expectLine(page, seeded);
    await expect(column(page, seeded[1]).getByText("Set by hand", { exact: true })).toBeVisible();
    // move-release: A version set by hand would end up too low.
    await openMenu(page, seeded[1]);
    await expect(menuItem(page, "Move later")).toBeDisabled();
    await expect(menuItem(page, "Move later")).toContainText(
      "Then v1.0.1 Membership hardening is not higher than v1.1.0 Account and profile, the release before it",
    );
    await page.keyboard.press("Escape");
    await expectLine(page, seeded);

    // Notifications set by hand to v1.2.5.
    let dialog = await openEdit(page, seeded[3]);
    await choice(dialog, "Custom").click();
    await dialog.getByLabel("Custom version").fill("v1.2.5");
    await dialog.getByRole("button", { name: "Save release", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expectLine(page, [seeded[0], seeded[1], seeded[2], "1.2.5 Notifications"]);

    // edit-release: Notifications set by hand after Account and profile.
    dialog = await openEdit(page, seeded[2]);
    await choice(dialog, "Custom").click();
    await dialog.getByLabel("Custom version").fill("v1.3.0");
    await expect(
      dialog.getByText(
        "Then v1.2.5 Notifications is not higher than v1.3.0 Account and profile, the release before it. Set a different version.",
        { exact: true },
      ),
    ).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Save release", exact: true })).toBeDisabled();
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(dialog).toHaveCount(0);

    // move-release: A version set by hand keeps its number.
    await openMenu(page, "1.2.5 Notifications");
    await menuItem(page, "Move earlier").click();
    const kept = [seeded[0], seeded[1], "1.2.5 Notifications", "1.3.0 Account and profile"];
    await expectLine(page, kept);
    await expect(column(page, kept[2]).getByText("Set by hand", { exact: true })).toBeVisible();
    await openWorklane(page, slug, agenda, "Agenda and publication");
    await expectLine(page, kept);

    // edit-release: Back to Minor.
    dialog = await openEdit(page, kept[2]);
    await choice(dialog, "Minor").click();
    await expect(
      dialog.getByText("Follows v1.0.1 Membership hardening. Moving it renumbers it.", { exact: true }),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "Save release", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    const minor = [seeded[0], seeded[1], "1.1.0 Notifications", "1.2.0 Account and profile"];
    await expectLine(page, minor);
    await expect(column(page, minor[2]).getByText("Set by hand", { exact: true })).toHaveCount(0);

    // edit-release: A new title and description.
    dialog = await openEdit(page, minor[1]);
    await dialog.getByLabel("Title", { exact: true }).fill("Memberships");
    await dialog.getByLabel("Description", { exact: true }).fill(MEMBERSHIPS);
    await dialog.getByRole("button", { name: "Save release", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    for (const [id, name] of [[fan, "Fan accounts and push"], [main, "main"]]) {
      await openWorklane(page, slug, id, name);
      await expect(column(page, "1.0.1 Memberships").getByText(MEMBERSHIPS, { exact: true })).toBeVisible();
    }

    // repository-stops-releasing: appboypov/rebl has planned releases.
    let edit = await openEditRepository(page, context.projectUrl, REBL);
    await chooseOption(page, edit.getByLabel("Releases", { exact: true }), "This repository does not release");
    await expect(edit.getByRole("alert")).toHaveText(
      "Remove its unshipped releases first: v1.0.0 First production release, v1.0.1 Memberships, v1.1.0 Notifications, v1.2.0 Account and profile.",
    );
    await expect(edit.getByRole("button", { name: "Save", exact: true })).toBeDisabled();
    await edit.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(edit).toHaveCount(0);

    // remove-release: The last release removed.
    const line = await client.planRelease(rebl, { title: "Later", bump: "patch" });
    for (const release of line) {
      if (release.title !== "Account and profile") await client.removeRelease(release.id);
    }
    await openWorklane(page, slug, fan, "Fan accounts and push");
    await expectLine(page, ["0.1.0 Account and profile"]);
    await openMenu(page, "0.1.0 Account and profile");
    await menuItem(page, "Remove release").click();
    const remove = page.getByRole("alertdialog", { name: "Remove v0.1.0 Account and profile?", exact: true });
    await remove.getByRole("button", { name: "Remove release", exact: true }).click();
    await expect(remove).toHaveCount(0);
    await expect(timeline(page).getByText("No releases yet", { exact: true })).toBeVisible();
    await expect(timeline(page).getByRole("button", { name: "Plan release", exact: true })).toBeVisible();

    // repository-stops-releasing: Releases removed first.
    edit = await openEditRepository(page, context.projectUrl, REBL);
    await chooseOption(page, edit.getByLabel("Releases", { exact: true }), "This repository does not release");
    await expect(edit.getByText("Its worklanes plan no releases.", { exact: true })).toBeVisible();
    await edit.getByRole("button", { name: "Save", exact: true }).click();
    await expect(edit).toHaveCount(0);
    // worklane-release-chips: Does not release.
    const card = await openCard(page, REBL);
    await expect(card.getByText("Does not release", { exact: true })).toHaveCount(3);

    // worklane-release-line: A repository that does not release shows so on its worklanes.
    await openWorklane(page, slug, fan, "Fan accounts and push");
    await expect(
      timeline(page).getByText(`${REBL} does not release, so its worklanes plan no releases.`, { exact: true }),
    ).toBeVisible();
    await expect(timeline(page).getByRole("button", { name: "Plan release", exact: true })).toHaveCount(0);
    await expect(page.getByText("does not release", { exact: true })).toBeVisible();
  });

  test("appboypov/rebl-admin plans after its last tag, and releases again", async ({ page }) => {
    test.setTimeout(90000);
    if (!api) throw new Error("Test API setup did not finish");
    const client = api;
    const context = await setupProject(page, client, ADMIN, "/home/e2e/Repos/rebl-admin");
    const { slug } = context;
    const main = (await client.listRepositoryWorklanes(context.repository.id)).find((worklane) => worklane.is_main);
    if (!main) throw new Error("appboypov/rebl-admin has no main worklane");

    // worklane-release-chips: No releases.
    await page.goto(context.projectUrl, { waitUntil: "domcontentloaded" });
    const card = await openCard(page, ADMIN);
    await expect(worklaneRow(card, "main").getByText("No releases planned", { exact: true })).toBeVisible();

    // plan-release: The first release after the last tag.
    await openWorklane(page, slug, main.id, "main");
    let dialog = await openPlan(page, ADMIN);
    await expectOptions(dialog, "v0.4.3", "v0.5.0", "v1.0.0");
    await expect(dialog.getByText("Follows v0.4.2, the last tag. Moving it renumbers it.", { exact: true })).toBeVisible();
    // plan-release: Too low after the last tag.
    await dialog.getByLabel("Title", { exact: true }).fill("Admin console");
    await choice(dialog, "Custom").click();
    await dialog.getByLabel("Custom version").fill("v0.4.2");
    await expect(dialog.getByText("Must be higher than v0.4.2, the last tag.", { exact: true })).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Plan v0.4.2", exact: true })).toBeDisabled();
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(dialog).toHaveCount(0);

    // repository-stops-releasing: Releasing again.
    await client.updateRepository(context.repository.id, "none");
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(
      timeline(page).getByText(`${ADMIN} does not release, so its worklanes plan no releases.`, { exact: true }),
    ).toBeVisible({ timeout: 15000 });
    const edit = await openEditRepository(page, context.projectUrl, ADMIN);
    await chooseOption(page, edit.getByLabel("Releases", { exact: true }), "Tags only · tags v{version}");
    await edit.getByRole("button", { name: "Save", exact: true }).click();
    await expect(edit).toHaveCount(0);
    await openWorklane(page, slug, main.id, "main");
    dialog = await openPlan(page, ADMIN);
    await expectOptions(dialog, "v0.4.3", "v0.5.0", "v1.0.0");
  });
});
