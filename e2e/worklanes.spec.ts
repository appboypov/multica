import { test, expect, type Locator, type Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { setTimeout as delay } from "node:timers/promises";
import { createTestApi, loginAsDefault, waitForPageText } from "./helpers";
import { githubRepository, startGitHubFake, stopGitHubFake, type FakeGitHub } from "./github-fake";
import type {
  TestApiClient, TestRepositoryCheckout, TestRepositoryCheckoutResult,
  TestWorklaneAnswer, TestWorklaneProgress, TestWorklaneStep,
} from "./fixtures";

const REBL = "appboypov/rebl";
const ADMIN = "appboypov/rebl-admin";
const WIDGETS = "appboypov/fan-widgets";
const OVERVIEW = "appboypov/das-overview";
const PRANK = "appboypov/photoprank";
const WORKTREES = ["worklane-worktree-v1"];
const ACTOR = "E2E User";
const LOCKFILE_ERROR =
  "ERR_PNPM_OUTDATED_LOCKFILE  Cannot install with \"frozen-lockfile\" because pnpm-lock.yaml is not up to date with package.json";

/** The runtime expands `~` to its user's home, as the real daemon does. */
function resolved(path: string): string {
  return path.startsWith("~/") ? `/home/e2e/${path.slice(2)}` : path;
}

function answerCheckout(request: TestRepositoryCheckout): TestRepositoryCheckoutResult {
  const installCommands: Record<string, { file: string; command: string }[]> = {
    [REBL]: [{ file: "pnpm-lock.yaml", command: "pnpm install" }],
    [ADMIN]: [{ file: "pnpm-lock.yaml", command: "pnpm install" }],
    [WIDGETS]: [{ file: "pubspec.yaml", command: "flutter pub get" }, { file: "uv.lock", command: "uv sync" }],
    [PRANK]: [{ file: "pnpm-lock.yaml", command: "pnpm install" }],
  };
  return {
    status: "completed",
    resolved_path: resolved(request.path),
    passed: true,
    failures: [],
    version_files: ["package.json"],
    last_tag: "v0.4.2",
    install_commands: installCommands[request.full_name] ?? [],
  };
}

const STEP_KEYS = ["fetch", "add_worktree", "install", "open"] as const;

/**
 * The daemon side of worklane requests on one runtime, through the real
 * heartbeat and progress routes. It keeps which worktree paths are on disk,
 * and the test makes a step fail, a path hold something else, or a remove be
 * refused. While `paused`, each step waits after it reports running until the
 * test calls `advance()`, so the dialog can be seen at every step.
 */
class FakeWorklaneDaemon {
  readonly requests: { kind: string; path: string; from_step?: string; install_command?: string }[] = [];
  readonly onDisk = new Map<string, "worktree" | "other">();
  readonly fetchFails = new Set<string>();
  readonly installFails = new Set<string>();
  readonly removeRefusals = new Map<string, { code: string; detail: string }>();
  paused = false;
  private holds: (() => void)[] = [];

  async advance() {
    const deadline = Date.now() + 15000;
    while (this.holds.length === 0) {
      if (Date.now() > deadline) throw new Error("No worklane step is waiting");
      await delay(50);
    }
    this.holds.shift()?.();
  }

  private hold(signal: AbortSignal): Promise<void> {
    if (!this.paused) return Promise.resolve();
    const { promise, resolve, reject } = Promise.withResolvers<void>();
    this.holds.push(resolve);
    signal.addEventListener("abort", () => reject(signal.reason), { once: true });
    return promise;
  }

  readonly answer: TestWorklaneAnswer = async (request, report, signal) => {
    this.requests.push(request);
    const path = resolved(request.path);
    if (request.kind === "remove") {
      const refusal = this.removeRefusals.get(path);
      if (refusal) {
        await report({ outcome: "refused", reason: refusal });
        return;
      }
      this.onDisk.delete(path);
      await report({ outcome: "removed" });
      return;
    }
    if (request.kind !== "create") throw new Error(`Unexpected worklane request ${request.kind}`);

    const folder = path.split("/").at(-1) ?? "";
    const from = request.from_step ? STEP_KEYS.indexOf(request.from_step as (typeof STEP_KEYS)[number]) : 0;
    const steps: TestWorklaneStep[] = STEP_KEYS.map((key, index) => ({
      key, state: index < from ? "done" : "pending", detail: "", output: "", duration_ms: 0,
    }));
    let reportedPath = "";
    const send = (reason?: TestWorklaneProgress["reason"]) => report({
      steps: steps.map((step) => ({ ...step })),
      ...(reportedPath ? { path: reportedPath } : {}),
      ...(reason ? { reason } : {}),
    });
    const start = async (step: TestWorklaneStep) => {
      step.state = "running";
      await send();
      await this.hold(signal);
    };
    const [fetch, addWorktree, install] = steps;
    if (!fetch || !addWorktree || !install) throw new Error("Missing create steps");

    if (from <= 0) {
      await start(fetch);
      if (this.fetchFails.has(folder)) {
        Object.assign(fetch, {
          state: "failed",
          detail: `fatal: unable to access 'https://github.com/${request.full_name}.git/': Could not resolve host: github.com`,
        });
        await send({ code: "fetch_failed", detail: fetch.detail });
        return;
      }
      Object.assign(fetch, { state: "done", duration_ms: 1200 });
    }
    if (from <= 1) {
      await start(addWorktree);
      const there = this.onDisk.get(path);
      if (there === "other") {
        Object.assign(addWorktree, {
          state: "failed",
          detail: `${path} holds something that is not a checkout of ${request.full_name}. Move it or pick another folder.`,
        });
        await send({ code: "path_not_checkout", detail: addWorktree.detail });
        return;
      }
      this.onDisk.set(path, "worktree");
      reportedPath = path;
      Object.assign(addWorktree, { state: "done", detail: there === "worktree" ? "Uses the worktree that is there" : "" });
    }
    if (!request.install_command) {
      install.state = "skipped";
      await send();
      return;
    }
    await start(install);
    if (this.installFails.has(folder)) {
      Object.assign(install, {
        state: "failed", detail: `${request.install_command} exited with 1`, output: LOCKFILE_ERROR,
      });
      await send({ code: "install_failed", detail: install.detail });
      return;
    }
    install.state = "done";
    await send();
  };
}

function cardOf(page: Page, fullName: string) {
  const toggle = page.getByRole("button", { name: `Fold or unfold ${fullName}`, exact: true });
  return { toggle, card: page.locator("section").filter({ has: toggle }) };
}

async function openCard(page: Page, fullName: string): Promise<Locator> {
  const { toggle, card } = cardOf(page, fullName);
  await expect(toggle).toBeVisible({ timeout: 15000 });
  if (await toggle.getAttribute("aria-expanded") !== "true") await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  return card;
}

function newWorklaneDialog(page: Page, fullName: string): Locator {
  return page.getByRole("dialog", { name: `New worklane · ${fullName}`, exact: true });
}

async function openNewWorklane(page: Page, fullName: string): Promise<Locator> {
  const card = await openCard(page, fullName);
  await card.getByRole("button", { name: "Add worklane", exact: true }).click();
  const dialog = newWorklaneDialog(page, fullName);
  await expect(dialog).toBeVisible();
  return dialog;
}

async function openAddRepository(page: Page, projectName: string): Promise<Locator> {
  await page.getByRole("button", { name: "Repository", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: `Add repository · ${projectName}` });
  await expect(dialog).toBeVisible();
  return dialog;
}

async function checkRepository(dialog: Locator, fullName: string, path: string) {
  await dialog.getByRole("option", { name: `${fullName} Private · main`, exact: true }).click();
  await dialog.getByLabel("Path", { exact: true }).fill(path);
  await dialog.getByLabel("Path", { exact: true }).press("Enter");
  await expect(dialog.getByText(`Git checkout of ${fullName}`, { exact: true })).toBeVisible({ timeout: 15000 });
}

/**
 * Picks an option of an open-able select from the keyboard: the popup scrolls
 * its list under the pointer, so a pointer click never finds the option stable.
 */
async function chooseOption(page: Page, trigger: Locator, name: string) {
  await trigger.click();
  const option = page.getByRole("option", { name, exact: true });
  await expect(option).toBeAttached();
  for (let i = 0; i < 10 && await option.getAttribute("data-highlighted") === null; i++) {
    await page.keyboard.press("ArrowDown");
  }
  await expect(option).toHaveAttribute("data-highlighted", "");
  await page.keyboard.press("Enter");
  await expect(option).toHaveCount(0);
}

async function openRepositoryMenuItem(page: Page, fullName: string, item: string) {
  const { card } = cardOf(page, fullName);
  await card.getByRole("button", { name: "Repository actions", exact: true }).click();
  await page.getByRole("menuitem", { name: item, exact: true }).click();
}

async function openWorklaneMenuItem(page: Page, scope: Locator, item: "Rename" | "Remove") {
  await scope.getByRole("button", { name: "Worklane actions", exact: true }).click();
  await page.getByRole("menuitem", { name: item, exact: true }).click();
}

function worklaneRow(card: Locator, name: string): Locator {
  // The inner locator resolves inside each listitem, so it starts from the page.
  return card.getByRole("listitem").filter({ has: card.page().getByRole("link", { name: new RegExp(`^${name}\\s*Free`) }) });
}

function step(dialog: Locator, label: string): Locator {
  return dialog.getByRole("listitem").filter({ hasText: label });
}

async function expectStep(dialog: Locator, label: string, state: "Pending" | "Running" | "Done" | "Skipped" | "Failed") {
  await expect(step(dialog, label).getByLabel(state, { exact: true })).toBeVisible({ timeout: 15000 });
}

test.describe("Worklanes of a repository / UJ-3", () => {
  let github: FakeGitHub;
  let api: TestApiClient | undefined;
  let suffix: string;

  test.beforeAll(async () => { github = await startGitHubFake(); });
  test.afterAll(async () => { await stopGitHubFake(github); });
  test.beforeEach(async () => {
    suffix = `${Date.now().toString(36)}-${randomUUID().slice(0, 8)}`;
    github.repositories.splice(
      0, github.repositories.length,
      ...[REBL, ADMIN, WIDGETS, OVERVIEW, PRANK].map((name, index) => githubRepository(name, 920001 + index)),
    );
    api = await createTestApi();
    await api.seedGitHubInstallation(Date.now());
  });
  test.afterEach(async () => { await api?.cleanup(); });

  /**
   * Rebl Records (key `rebl`) holds Rebl, which holds appboypov/rebl with
   * install command `pnpm install`, checked out on Brian's Mac, a runtime
   * that can make worktrees.
   */
  async function setupRebl(page: Page, client: TestApiClient) {
    const organisationName = `Rebl Records ${suffix}`;
    const organisation = await client.createOrganisation(organisationName, "rebl");
    const projectName = `Rebl ${suffix}`;
    const project = await client.createProject(projectName, { organisation_id: organisation.id });
    const runtimeName = `Brian's Mac ${suffix}`;
    const runtime = await client.registerRepositoryRuntime(runtimeName, "online", WORKTREES);
    const daemon = new FakeWorklaneDaemon();
    let stopAnswering = await client.startRepositoryCheckoutAnswerer(runtime.id, answerCheckout, undefined, daemon.answer);
    const check = await client.checkRepositoryCheckout(runtime.id, REBL, "/home/e2e/Repos/rebl");
    const rebl = await client.createRepository(project.id, REBL, check, "pnpm install");
    const slug = await loginAsDefault(page);
    const projectUrl = `/${slug}/projects/${project.id}`;
    await page.goto(projectUrl, { waitUntil: "domcontentloaded" });
    await waitForPageText(page, projectName);
    return {
      organisation, organisationName, project, projectName, runtime, runtimeName, daemon, rebl, slug, projectUrl,
      /** The runtime stops answering and shows offline. */
      async goOffline() {
        await stopAnswering();
        await client.setRuntimeStatus(runtime.id, "offline");
      },
      async goOnline() {
        await client.setRuntimeStatus(runtime.id, "online");
        stopAnswering = await client.startRepositoryCheckoutAnswerer(runtime.id, answerCheckout, undefined, daemon.answer);
      },
    };
  }

  test("Install command: found, two files, none, typed, set to None and a repository from before", async ({ page }) => {
    test.setTimeout(180000);
    if (!api) throw new Error("Test API setup did not finish");
    const client = api;
    const context = await setupRebl(page, client);

    // repository-install-command: A pnpm repository.
    const add = await openAddRepository(page, context.projectName);
    await checkRepository(add, ADMIN, "/home/e2e/Repos/rebl-admin");
    const install = add.getByLabel("Install", { exact: true });
    await expect(install).toContainText("pnpm install");

    // repository-install-command: Typed by hand.
    await chooseOption(page, install, "Type a command");
    await add.getByLabel("Type a command", { exact: true }).fill("pnpm install --frozen-lockfile");
    await add.getByRole("button", { name: "Add repository", exact: true }).click();
    await expect(add).toHaveCount(0);
    const admin = (await client.listRepositories()).find((repository) => repository.full_name === ADMIN);
    expect(admin?.install_command).toBe("pnpm install --frozen-lockfile");
    await openRepositoryMenuItem(page, ADMIN, "Edit repository");
    const editAdmin = page.getByRole("dialog", { name: "Edit repository", exact: true });
    await expect(editAdmin.getByLabel("Type a command", { exact: true })).toHaveValue("pnpm install --frozen-lockfile");
    await editAdmin.getByRole("button", { name: "Cancel", exact: true }).click();
    const adminWorklane = await openNewWorklane(page, ADMIN);
    await expect(adminWorklane.getByText("pnpm install --frozen-lockfile", { exact: true })).toBeVisible();
    await adminWorklane.getByRole("button", { name: "Cancel", exact: true }).click();

    // repository-install-command: Two files.
    const widgets = await openAddRepository(page, context.projectName);
    await checkRepository(widgets, WIDGETS, "/home/e2e/Repos/fan-widgets");
    await expect(widgets.getByLabel("Install", { exact: true })).toContainText("flutter pub get");
    await widgets.getByLabel("Install", { exact: true }).click();
    const uvSync = page.getByRole("option", { name: /^uv sync/ });
    await expect(uvSync).toContainText("from uv.lock");
    await expect(page.getByRole("option", { name: /^flutter pub get/ })).toContainText("from pubspec.yaml");
    await page.keyboard.press("Escape");
    await widgets.getByRole("button", { name: "Cancel", exact: true }).click();

    // repository-install-command: None of the files.
    const overview = await openAddRepository(page, context.projectName);
    await checkRepository(overview, OVERVIEW, "/home/e2e/Repos/das-overview");
    await expect(overview.getByLabel("Install", { exact: true })).toContainText("None");
    await expect(overview.getByText("New worklanes install nothing.", { exact: true })).toBeVisible();
    await overview.getByRole("button", { name: "Cancel", exact: true }).click();

    // repository-install-command: Set to None.
    await openRepositoryMenuItem(page, REBL, "Edit repository");
    const editRebl = page.getByRole("dialog", { name: "Edit repository", exact: true });
    await expect(editRebl.getByLabel("Install", { exact: true })).toContainText("pnpm install");
    await chooseOption(page, editRebl.getByLabel("Install", { exact: true }), "None");
    await editRebl.getByRole("button", { name: "Save", exact: true }).click();
    await expect(editRebl).toHaveCount(0);
    const none = await openNewWorklane(page, REBL);
    await expect(none.getByText("Installs nothing", { exact: true })).toBeVisible();
    await none.getByRole("button", { name: "Cancel", exact: true }).click();
    expect((await client.listRepositories()).find((repository) => repository.id === context.rebl.id)?.install_command).toBe("");

    // repository-install-command: A repository from before.
    await client.clearRepositoryInstallCommand(context.rebl.id);
    await page.reload({ waitUntil: "domcontentloaded" });
    await openRepositoryMenuItem(page, REBL, "Edit repository");
    await expect(editRebl.getByLabel("Install", { exact: true })).toContainText("pnpm install", { timeout: 15000 });
    await editRebl.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect.poll(async () => (await client.listRepositories()).find((repository) => repository.id === context.rebl.id)?.install_command)
      .toBe("pnpm install");
  });

  test("New worklane: names, folders, runtimes and the worktree", async ({ page }) => {
    test.setTimeout(180000);
    if (!api) throw new Error("Test API setup did not finish");
    const client = api;
    const context = await setupRebl(page, client);
    const { daemon, runtime, runtimeName } = context;
    await client.createWorklane(context.rebl.id, "Fan accounts and push", "fan-accounts-and-push", runtime.daemon_id);
    const adminCheck = await client.checkRepositoryCheckout(runtime.id, ADMIN, "/home/e2e/Repos/rebl-admin");
    const admin = await client.createRepository(context.project.id, ADMIN, adminCheck, "pnpm install");
    await client.createWorklane(admin.id, "Fan app", "fan-app", runtime.daemon_id);
    const studioName = `Studio Mac mini ${suffix}`;
    const studio = await client.registerRepositoryRuntime(studioName, "online", WORKTREES);
    await client.startRepositoryCheckoutAnswerer(studio.id, answerCheckout);
    await page.reload({ waitUntil: "domcontentloaded" });

    // new-worklane: From the repository card.
    const dialog = await openNewWorklane(page, REBL);
    const name = dialog.getByLabel("Name", { exact: true });
    const folder = dialog.getByLabel("Folder", { exact: true });
    const create = dialog.getByRole("button", { name: "Create worklane", exact: true });

    // new-worklane: A taken name.
    await name.fill("fan accounts and push");
    await expect(dialog.getByRole("alert")).toHaveText(`${REBL} already has a worklane fan accounts and push`);
    await expect(create).toBeDisabled();
    // new-worklane: main is taken.
    await name.fill("Main");
    await expect(dialog.getByRole("alert")).toHaveText(`${REBL} already has a worklane Main`);
    await expect(create).toBeDisabled();
    // new-worklane: The same name in another repository · Filled in.
    await name.fill("Fan app");
    await expect(dialog.getByRole("alert")).toHaveCount(0);
    await expect(folder).toHaveValue("fan-app");
    await expect(dialog.getByText(`Unique among ${REBL}'s worklanes`, { exact: true })).toBeVisible();

    // new-worklane: Brian's Mac holds the checkout.
    const runtimeField = dialog.getByLabel("Runtime", { exact: true });
    await expect(runtimeField).toContainText(`${runtimeName} · online`);
    await expect(dialog.getByText("Online runtimes that can make worktrees", { exact: true })).toBeVisible();
    // new-worklane: A runtime without the checkout.
    await runtimeField.click();
    const studioOption = page.getByRole("option", { name: new RegExp(`^${studioName} · online`) });
    await expect(studioOption).toContainText(`No checkout of ${REBL}`);
    await expect(studioOption).toBeDisabled();
    await page.keyboard.press("Escape");
    await expect(runtimeField).toContainText(`${runtimeName} · online`);

    // new-worklane: Fan app in Rebl.
    await expect(dialog.getByText("~/Worktrees/rebl/rebl/fan-app", { exact: true })).toBeVisible();
    await expect(dialog.getByText("from origin/main", { exact: true })).toBeVisible();
    await expect(dialog.getByText("pnpm install", { exact: true })).toBeVisible();
    await expect(create).toBeEnabled();

    // new-worklane: Edited.
    await folder.fill("fans");
    await name.fill("Fan app v2");
    await expect(folder).toHaveValue("fans");
    await expect(dialog.getByText("~/Worktrees/rebl/rebl/fans", { exact: true })).toBeVisible();
    // new-worklane: Not one folder name.
    await folder.fill("Fan/App");
    await expect(dialog.getByText("One folder name: lowercase letters, digits and dashes", { exact: true })).toBeVisible();
    await expect(create).toBeDisabled();
    // new-worklane: A folder in use.
    await folder.fill("fan-accounts-and-push");
    await expect(dialog.getByText("Already used by Fan accounts and push", { exact: true })).toBeVisible();
    await expect(create).toBeDisabled();
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(dialog).toHaveCount(0);

    // new-worklane: From Projects.
    await page.goto(`/${context.slug}/projects`, { waitUntil: "domcontentloaded" });
    const projectFold = page.getByRole("button", { name: `Fold or unfold the repositories of ${context.projectName}`, exact: true });
    await expect(projectFold).toBeVisible({ timeout: 15000 });
    if (await projectFold.getAttribute("aria-expanded") !== "true") await projectFold.click();
    await page.getByRole("button", { name: `Show the worklanes of ${REBL}`, exact: true }).click();
    await page.getByRole("button", { name: `New worklane in ${REBL}`, exact: true }).click();
    const fromProjects = newWorklaneDialog(page, REBL);
    await expect(fromProjects).toBeVisible();
    await fromProjects.getByRole("button", { name: "Cancel", exact: true }).click();

    // new-worklane: A project without an organisation · No install command · Nothing to install.
    const overviewName = `das-overview ${suffix}`;
    const overviewProject = await client.createProject(overviewName);
    const overviewCheck = await client.checkRepositoryCheckout(runtime.id, OVERVIEW, "/home/e2e/Repos/das-overview");
    await client.createRepository(overviewProject.id, OVERVIEW, overviewCheck, "");
    await page.goto(`/${context.slug}/projects/${overviewProject.id}`, { waitUntil: "domcontentloaded" });
    const docs = await openNewWorklane(page, OVERVIEW);
    await docs.getByLabel("Name", { exact: true }).fill("Docs");
    await expect(docs.getByText("~/Worktrees/das-overview/docs", { exact: true })).toBeVisible();
    await expect(docs.getByText("Installs nothing", { exact: true })).toBeVisible();
    daemon.paused = true;
    await docs.getByRole("button", { name: "Create worklane", exact: true }).click();
    await expectStep(docs, "Fetch origin/main", "Running");
    await expect(step(docs, "Install dependencies")).toContainText("Skipped");
    await daemon.advance();
    await daemon.advance();
    await expect(page.getByRole("heading", { name: "Docs", level: 1 })).toBeVisible({ timeout: 15000 });
    daemon.paused = false;
    expect(daemon.requests.at(-1)?.install_command ?? "").toBe("");

    // new-worklane: A runtime that cannot make worktrees.
    const oldName = `Old Mac ${suffix}`;
    const old = await client.registerRepositoryRuntime(oldName, "online");
    await client.startRepositoryCheckoutAnswerer(old.id, answerCheckout);
    const prankProject = await client.createProject(`Photo Prank ${suffix}`);
    const prankCheck = await client.checkRepositoryCheckout(old.id, PRANK, "/home/e2e/Repos/photoprank");
    await client.createRepository(prankProject.id, PRANK, prankCheck, "pnpm install");
    await page.goto(`/${context.slug}/projects/${prankProject.id}`, { waitUntil: "domcontentloaded" });
    const prank = await openNewWorklane(page, PRANK);
    await prank.getByLabel("Name", { exact: true }).fill("Fan app");
    await prank.getByLabel("Runtime", { exact: true }).click();
    const oldOption = page.getByRole("option", { name: new RegExp(`^${oldName} · online`) });
    await expect(oldOption).toContainText("Cannot make worktrees");
    await expect(oldOption).toBeDisabled();
    await page.keyboard.press("Escape");
    await expect(prank.getByRole("button", { name: "Create worklane", exact: true })).toBeDisabled();
    await prank.getByRole("button", { name: "Cancel", exact: true }).click();

    // new-worklane: The checkout's runtime is offline.
    await context.goOffline();
    await page.goto(context.projectUrl, { waitUntil: "domcontentloaded" });
    const offline = await openNewWorklane(page, REBL);
    await offline.getByLabel("Name", { exact: true }).fill("Fan app");
    await expect(offline.getByLabel("Runtime", { exact: true })).toContainText(`${runtimeName} · offline`);
    await expect(offline.getByRole("button", { name: "Create worklane", exact: true })).toBeDisabled();
    await offline.getByRole("button", { name: "Cancel", exact: true }).click();
  });

  test("Creating Fan app step by step, a failed fetch and install, Try again and a changed key", async ({ page }) => {
    test.setTimeout(240000);
    if (!api) throw new Error("Test API setup did not finish");
    const client = api;
    const context = await setupRebl(page, client);
    const { daemon, runtime, runtimeName } = context;
    await client.createWorklane(context.rebl.id, "Fan accounts and push", "fan-accounts-and-push", runtime.daemon_id);
    await page.reload({ waitUntil: "domcontentloaded" });
    const fanAppPath = "/home/e2e/Worktrees/rebl/rebl/fan-app";

    // new-worklane: Fetch fails.
    daemon.fetchFails.add("fan-app");
    const fetchFails = await openNewWorklane(page, REBL);
    await fetchFails.getByLabel("Name", { exact: true }).fill("Fan app");
    await fetchFails.getByRole("button", { name: "Create worklane", exact: true }).click();
    await expectStep(fetchFails, "Fetch origin/main", "Failed");
    await expect(fetchFails.getByRole("alert")).toContainText("Could not resolve host: github.com");
    await expect(fetchFails.getByRole("alert")).toContainText("No worklane is saved until every step passes.");
    await expect(fetchFails.getByRole("button", { name: "Try again", exact: true })).toBeEnabled();
    expect(daemon.onDisk.has(fanAppPath)).toBe(false);
    await fetchFails.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(fetchFails).toHaveCount(0);
    daemon.fetchFails.clear();

    // new-worklane: Fan app is created, step by step.
    daemon.paused = true;
    const dialog = await openNewWorklane(page, REBL);
    await dialog.getByLabel("Name", { exact: true }).fill("Fan app");
    await dialog.getByRole("button", { name: "Create worklane", exact: true }).click();
    await expect(dialog.getByText(`Creating on ${runtimeName}`, { exact: true })).toBeVisible();
    await expectStep(dialog, "Fetch origin/main", "Running");
    await expectStep(dialog, "Add worktree", "Pending");
    await expect(step(dialog, "Add worktree")).toContainText("~/Worktrees/rebl/rebl/fan-app");
    await expectStep(dialog, "Install dependencies", "Pending");
    await expect(step(dialog, "Install dependencies")).toContainText("pnpm install");
    await expectStep(dialog, "Open the worklane", "Pending");
    await expect(dialog.getByRole("button", { name: "Cancel", exact: true })).toBeDisabled();
    await expect(dialog.getByRole("button", { name: "Creating worklane", exact: true })).toBeDisabled();
    await daemon.advance();
    await expectStep(dialog, "Fetch origin/main", "Done");
    await expect(step(dialog, "Fetch origin/main")).toContainText("1.2 s");
    await expectStep(dialog, "Add worktree", "Running");
    await daemon.advance();
    await expectStep(dialog, "Add worktree", "Done");
    await expect(step(dialog, "Add worktree")).toContainText(fanAppPath);
    await expectStep(dialog, "Install dependencies", "Running");
    await expect(dialog.getByRole("button", { name: "Cancel", exact: true })).toBeDisabled();
    await daemon.advance();
    daemon.paused = false;

    // new-worklane: Fan app opens · worklane-page: A new worklane, Created by Brian.
    await expect(dialog).toHaveCount(0, { timeout: 15000 });
    await expect(page.getByRole("heading", { name: "Fan app", level: 1 })).toBeVisible();
    await expect(page).toHaveURL(/\/worklanes\/[0-9a-f-]+$/);
    const crumbs = page.locator("header").filter({ has: page.getByRole("button", { name: "Worklane actions", exact: true }) });
    await expect(crumbs.getByRole("link", { name: "Projects", exact: true })).toBeVisible();
    await expect(crumbs.getByRole("link", { name: context.projectName, exact: true })).toBeVisible();
    await expect(crumbs.getByRole("link", { name: REBL, exact: true })).toBeVisible();
    await expect(crumbs.getByText("Fan app", { exact: true })).toBeVisible();
    const title = page.locator("header").filter({ has: page.getByRole("heading", { level: 1 }) });
    await expect(title.getByLabel("Repository", { exact: true })).toHaveText(REBL);
    await expect(title.getByText("Free", { exact: true })).toBeVisible();
    await expect(page.getByText("No releases yet", { exact: true })).toBeVisible();
    const activity = page.getByRole("region", { name: "Activity", exact: true });
    await expect(activity.getByRole("listitem")).toHaveCount(1);
    await expect(activity.getByRole("listitem")).toContainText(`${ACTOR} created Fan app on ${runtimeName}`);
    const fanApp = (await client.listRepositoryWorklanes(context.rebl.id)).find((worklane) => worklane.name === "Fan app");
    expect(fanApp?.path).toBe(fanAppPath);
    expect(fanApp?.state).toBe("free");

    // new-worklane: Fan app opens, after the repository's other worklanes · repository-worklanes: A worklane created.
    await page.goto(context.projectUrl, { waitUntil: "domcontentloaded" });
    const card = await openCard(page, REBL);
    await expect(card.getByRole("listitem")).toHaveText([/^main\s*Free/, /^Fan accounts and push\s*Free/, /^Fan app\s*Free/]);
    await expect(card.getByText("3 worklanes", { exact: true })).toBeVisible();

    // new-worklane: Install fails.
    const agendaPath = "/home/e2e/Worktrees/rebl/rebl/agenda-and-publication";
    daemon.installFails.add("agenda-and-publication");
    const failing = await openNewWorklane(page, REBL);
    await failing.getByLabel("Name", { exact: true }).fill("Agenda and publication");
    await failing.getByRole("button", { name: "Create worklane", exact: true }).click();
    await expectStep(failing, "Install dependencies", "Failed");
    await expectStep(failing, "Fetch origin/main", "Done");
    await expectStep(failing, "Add worktree", "Done");
    await expectStep(failing, "Open the worklane", "Pending");
    await expect(step(failing, "Install dependencies")).toContainText("pnpm install exited with 1");
    await expect(step(failing, "Install dependencies")).toContainText("ERR_PNPM_OUTDATED_LOCKFILE");
    await expect(failing.getByRole("alert")).toContainText("No worklane is saved until every step passes.");
    await expect(failing.getByRole("button", { name: "Cancel", exact: true })).toBeEnabled();
    await expect(failing.getByRole("button", { name: "Try again", exact: true })).toBeEnabled();
    expect((await client.listRepositoryWorklanes(context.rebl.id)).map((worklane) => worklane.name)).not.toContain("Agenda and publication");

    // new-worklane: Cancel after a failure.
    await failing.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(failing).toHaveCount(0);
    await expect(worklaneRow(card, "Agenda and publication")).toHaveCount(0);
    await expect(card.getByText("3 worklanes", { exact: true })).toBeVisible();
    expect(daemon.onDisk.get(agendaPath)).toBe("worktree");

    // new-worklane: A worktree of the repository is there · Try again after the fix.
    const again = await openNewWorklane(page, REBL);
    await again.getByLabel("Name", { exact: true }).fill("Agenda and publication");
    await again.getByRole("button", { name: "Create worklane", exact: true }).click();
    await expectStep(again, "Install dependencies", "Failed");
    await expect(step(again, "Add worktree")).toContainText("Uses the worktree that is there");
    daemon.installFails.clear();
    await again.getByRole("button", { name: "Try again", exact: true }).click();
    await expect(again).toHaveCount(0, { timeout: 15000 });
    expect(daemon.requests.at(-1)?.from_step).toBe("install");
    await expect(page.getByRole("heading", { name: "Agenda and publication", level: 1 })).toBeVisible();
    // worklane-page: No entry for a failed creation.
    await expect(activity.getByRole("listitem")).toHaveCount(1);
    await expect(activity.getByRole("listitem")).toContainText(`${ACTOR} created Agenda and publication on ${runtimeName}`);

    // new-worklane: Something else is there.
    const notesPath = "/home/e2e/Worktrees/rebl/rebl/notes";
    daemon.onDisk.set(notesPath, "other");
    await page.goto(context.projectUrl, { waitUntil: "domcontentloaded" });
    const notes = await openNewWorklane(page, REBL);
    await notes.getByLabel("Name", { exact: true }).fill("Notes");
    await notes.getByRole("button", { name: "Create worklane", exact: true }).click();
    await expectStep(notes, "Add worktree", "Failed");
    await expect(notes.getByRole("alert")).toContainText(
      `${notesPath} holds something that is not a checkout of ${REBL}. Move it or pick another folder.`,
    );
    expect(daemon.onDisk.get(notesPath)).toBe("other");
    await notes.getByRole("button", { name: "Cancel", exact: true }).click();

    // new-worklane: A changed organisation key.
    await client.updateOrganisationKey(context.organisation.id, "rr");
    await page.reload({ waitUntil: "domcontentloaded" });
    const agenda = await openNewWorklane(page, REBL);
    await agenda.getByLabel("Name", { exact: true }).fill("Agenda");
    await expect(agenda.getByText("~/Worktrees/rr/rebl/agenda", { exact: true })).toBeVisible();
    await agenda.getByRole("button", { name: "Create worklane", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Agenda", level: 1 })).toBeVisible({ timeout: 15000 });
    const worklanes = await client.listRepositoryWorklanes(context.rebl.id);
    expect(worklanes.find((worklane) => worklane.name === "Agenda")?.path).toBe("/home/e2e/Worktrees/rr/rebl/agenda");
    expect(worklanes.find((worklane) => worklane.name === "Fan app")?.path).toBe(fanAppPath);
  });

  test("Worklane rows and counts, rename, remove and removing the repository", async ({ page }) => {
    test.setTimeout(240000);
    if (!api) throw new Error("Test API setup did not finish");
    const client = api;
    const context = await setupRebl(page, client);
    const { daemon, runtime, runtimeName } = context;
    await client.createWorklane(context.rebl.id, "Fan accounts and push", "fan-accounts-and-push", runtime.daemon_id);
    await client.createWorklane(context.rebl.id, "Agenda and publication", "agenda-and-publication", runtime.daemon_id);
    await page.reload({ waitUntil: "domcontentloaded" });

    // repository-worklanes: appboypov/rebl with three worklanes · Rebl Records (Rebl's page header).
    const card = await openCard(page, REBL);
    await expect(card.getByText("3 worklanes", { exact: true })).toBeVisible();
    await expect(card.getByRole("button", { name: "Add worklane", exact: true })).toBeVisible();
    await expect(card.getByRole("listitem")).toHaveText([/^main\s*Free/, /^Fan accounts and push\s*Free/, /^Agenda and publication\s*Free/]);
    await expect(page.getByText("1 repository · 3 worklanes", { exact: true })).toBeVisible();

    // repository-worklanes: Opening a worklane.
    await card.getByRole("link", { name: /^Fan accounts and push/ }).click();
    await expect(page.getByRole("heading", { name: "Fan accounts and push", level: 1 })).toBeVisible();

    // repository-worklanes: Rebl Records on Projects · Rebl on Projects.
    await page.goto(`/${context.slug}/projects`, { waitUntil: "domcontentloaded" });
    const organisationRow = page.getByRole("row").filter({ has: page.getByText(context.organisationName, { exact: true }) });
    await expect(organisationRow.getByText("1 project · 3 worklanes", { exact: true })).toBeVisible({ timeout: 15000 });
    await expect(page.getByText("1 repository · 3 worklanes", { exact: true })).toBeVisible();
    const projectFold = page.getByRole("button", { name: `Fold or unfold the repositories of ${context.projectName}`, exact: true });
    if (await projectFold.getAttribute("aria-expanded") !== "true") await projectFold.click();
    await expect(page.getByRole("button", { name: `New worklane in ${REBL}`, exact: true })).toBeVisible();
    await page.getByRole("button", { name: `Show the worklanes of ${REBL}`, exact: true }).click();
    const treeRows = page.getByRole("link", { name: /^(main|Fan accounts and push|Agenda and publication)\s*Free/ });
    await expect(treeRows).toHaveText([/^main/, /^Fan accounts and push/, /^Agenda and publication/]);

    // repository-worklanes: On the organisation's page · Rebl Records (its page).
    await page.goto(`/${context.slug}/organisations/${context.organisation.id}`, { waitUntil: "domcontentloaded" });
    await expect(page.getByText("3 worklanes", { exact: true }).first()).toBeVisible({ timeout: 15000 });
    const organisationFold = page.getByRole("button", { name: `Fold or unfold the repositories of ${context.projectName}`, exact: true });
    if (await organisationFold.getAttribute("aria-expanded") !== "true") await organisationFold.click();
    // The fold is persisted (`expandedRepositories`), so the row may still be open from Projects.
    const repositoryFold = page.getByRole("button", { name: new RegExp(`^(Show|Hide) the worklanes of ${REBL}$`) });
    if (await repositoryFold.getAttribute("aria-expanded") !== "true") await repositoryFold.click();
    await page.getByRole("link", { name: /^Fan accounts and push/ }).click();
    await expect(page.getByRole("heading", { name: "Fan accounts and push", level: 1 })).toBeVisible();

    // repository-worklanes: A worklane created, the counts after it.
    const fanAppId = await client.createWorklane(context.rebl.id, "Fan app", "fan-app", runtime.daemon_id);
    await page.goto(`/${context.slug}/projects`, { waitUntil: "domcontentloaded" });
    await expect(organisationRow.getByText("1 project · 4 worklanes", { exact: true })).toBeVisible({ timeout: 15000 });
    await expect(page.getByText("1 repository · 4 worklanes", { exact: true })).toBeVisible();
    await page.goto(`/${context.slug}/organisations/${context.organisation.id}`, { waitUntil: "domcontentloaded" });
    await expect(page.getByText("4 worklanes", { exact: true }).first()).toBeVisible({ timeout: 15000 });
    await page.goto(context.projectUrl, { waitUntil: "domcontentloaded" });
    await expect(page.getByText("1 repository · 4 worklanes", { exact: true })).toBeVisible();
    await expect(card.getByText("4 worklanes", { exact: true })).toBeVisible();

    // manage-worklanes: main.
    const mainRow = card.getByRole("listitem").first();
    await expect(mainRow).toHaveText(/^main/);
    await expect(mainRow.getByRole("button", { name: "Worklane actions", exact: true })).toHaveCount(0);
    await mainRow.getByRole("link").click();
    await expect(page.getByRole("heading", { name: "main", level: 1 })).toBeVisible();
    await expect(page.getByRole("button", { name: "Worklane actions", exact: true })).toHaveCount(0);

    // manage-worklanes: A taken name · Renaming Fan app · worklane-page: Renamed.
    await page.goto(`/${context.slug}/worklanes/${fanAppId}`, { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: "Fan app", level: 1 })).toBeVisible();
    await openWorklaneMenuItem(page, page.locator("body"), "Rename");
    const rename = page.getByRole("dialog", { name: "Rename worklane", exact: true });
    await rename.getByLabel("Name", { exact: true }).fill("agenda and publication");
    await expect(rename.getByRole("alert")).toHaveText(`${REBL} already has a worklane agenda and publication`);
    await expect(rename.getByRole("button", { name: "Rename", exact: true })).toBeDisabled();
    await rename.getByLabel("Name", { exact: true }).fill("Fan accounts");
    await rename.getByRole("button", { name: "Rename", exact: true }).click();
    await expect(rename).toHaveCount(0);
    await expect(page.getByRole("heading", { name: "Fan accounts", level: 1 })).toBeVisible();
    const activity = page.getByRole("region", { name: "Activity", exact: true });
    await expect(activity.getByRole("listitem")).toHaveText([
      new RegExp(`${ACTOR} created Fan app on ${runtimeName}`),
      new RegExp(`${ACTOR} renamed Fan app to Fan accounts`),
    ]);
    const fanAppPath = "/home/e2e/Worktrees/rebl/rebl/fan-app";
    const renamed = (await client.listRepositoryWorklanes(context.rebl.id)).find((worklane) => worklane.id === fanAppId);
    expect(renamed?.name).toBe("Fan accounts");
    expect(renamed?.path).toBe(fanAppPath);
    await page.goto(context.projectUrl, { waitUntil: "domcontentloaded" });
    await expect(card.getByRole("listitem")).toHaveText([
      /^main\s*Free/, /^Fan accounts and push\s*Free/, /^Agenda and publication\s*Free/, /^Fan accounts\s*Free/,
    ]);

    // manage-worklanes: Uncommitted work · Unmerged commits.
    await page.goto(`/${context.slug}/worklanes/${fanAppId}`, { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: "Fan accounts", level: 1 })).toBeVisible();
    await openWorklaneMenuItem(page, page.locator("body"), "Remove");
    const remove = page.getByRole("dialog", { name: "Remove Fan accounts?", exact: true });
    await expect(remove).toContainText(`This deletes ${fanAppPath} on ${runtimeName}`);
    daemon.removeRefusals.set(fanAppPath, { code: "uncommitted", detail: "" });
    await remove.getByRole("button", { name: "Remove", exact: true }).click();
    await expect(remove.getByRole("alert")).toHaveText("Fan accounts's worktree holds uncommitted changes", { timeout: 15000 });
    daemon.removeRefusals.set(fanAppPath, { code: "unpushed", detail: "1" });
    await remove.getByRole("button", { name: "Remove", exact: true }).click();
    await expect(remove.getByRole("alert")).toHaveText("Fan accounts's worktree holds commits that are not on origin/main", { timeout: 15000 });
    expect(daemon.onDisk.get(fanAppPath)).toBe("worktree");
    expect((await client.listRepositoryWorklanes(context.rebl.id)).map((worklane) => worklane.id)).toContain(fanAppId);
    await remove.getByRole("button", { name: "Cancel", exact: true }).click();

    // manage-worklanes: The runtime is offline.
    await context.goOffline();
    const removesBefore = daemon.requests.filter((request) => request.kind === "remove").length;
    await page.reload({ waitUntil: "domcontentloaded" });
    await openWorklaneMenuItem(page, page.locator("body"), "Remove");
    await remove.getByRole("button", { name: "Remove", exact: true }).click();
    await expect(remove.getByRole("alert")).toHaveText(`${runtimeName} is offline`);
    expect(daemon.requests.filter((request) => request.kind === "remove")).toHaveLength(removesBefore);
    await remove.getByRole("button", { name: "Cancel", exact: true }).click();
    await context.goOnline();

    // manage-worklanes: A clean worklane.
    daemon.removeRefusals.clear();
    await page.reload({ waitUntil: "domcontentloaded" });
    await openWorklaneMenuItem(page, page.locator("body"), "Remove");
    await remove.getByRole("button", { name: "Remove", exact: true }).click();
    await expect(remove).toHaveCount(0, { timeout: 15000 });
    await expect(page).toHaveURL(new RegExp(`${context.projectUrl}$`));
    await expect(card.getByRole("listitem")).toHaveText([/^main\s*Free/, /^Fan accounts and push\s*Free/, /^Agenda and publication\s*Free/]);
    await expect(card.getByText("3 worklanes", { exact: true })).toBeVisible();
    expect(daemon.onDisk.has(fanAppPath)).toBe(false);

    // Remove Agenda and publication from its row on the card.
    await openWorklaneMenuItem(page, worklaneRow(card, "Agenda and publication"), "Remove");
    const removeAgenda = page.getByRole("dialog", { name: "Remove Agenda and publication?", exact: true });
    await removeAgenda.getByRole("button", { name: "Remove", exact: true }).click();
    await expect(removeAgenda).toHaveCount(0, { timeout: 15000 });
    await expect(card.getByRole("listitem")).toHaveText([/^main\s*Free/, /^Fan accounts and push\s*Free/]);

    // manage-worklanes: appboypov/rebl with Fan app (here Fan accounts and push).
    await openRepositoryMenuItem(page, REBL, "Remove repository");
    await expect(page.getByText(`${REBL} still has the worklane Fan accounts and push`, { exact: true })).toBeVisible();
    await expect(cardOf(page, REBL).toggle).toBeVisible();
    expect((await client.listRepositories()).map((repository) => repository.id)).toContain(context.rebl.id);

    // manage-worklanes: After the worklane is removed.
    await openWorklaneMenuItem(page, worklaneRow(card, "Fan accounts and push"), "Remove");
    const removeLast = page.getByRole("dialog", { name: "Remove Fan accounts and push?", exact: true });
    await removeLast.getByRole("button", { name: "Remove", exact: true }).click();
    await expect(removeLast).toHaveCount(0, { timeout: 15000 });
    await expect(card.getByRole("listitem")).toHaveText([/^main\s*Free/]);
    await openRepositoryMenuItem(page, REBL, "Remove repository");
    await expect(cardOf(page, REBL).toggle).toHaveCount(0);
    expect((await client.listRepositories()).map((repository) => repository.id)).not.toContain(context.rebl.id);
  });
});
