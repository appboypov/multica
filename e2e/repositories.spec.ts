import { test, expect, type Locator, type Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { createTestApi, loginAsDefault, waitForPageText } from "./helpers";
import { githubRepository, startGitHubFake, stopGitHubFake, type FakeGitHub } from "./github-fake";
import type {
  TestApiClient, TestRepositoryCheckout, TestRepositoryCheckoutResult,
  TestDirectoryListing, TestDirectoryListingResult,
} from "./fixtures";

const ADMIN = "appboypov/rebl-admin";
const SITE = "appboypov/rebl-site";
const TAKEN = "appboypov/photoprank";
const WIDGETS = "appboypov/fan-widgets";
const VERSION = "Version in package.json · tags v{version}";
const TAGS = "Tags only · tags v{version}";
const NONE = "This repository does not release";
const MADSPEC_REPOS = ["appboypov/madspec", "appboypov/plannotator", "appboypov/omp-anytype-inbox"];

async function openAdd(page: Page, projectName: string): Promise<Locator> {
  await page.getByRole("button", { name: "Repository", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: `Add repository · ${projectName}` });
  await expect(dialog).toBeVisible();
  return dialog;
}

async function commitPath(dialog: Locator, path: string) {
  await dialog.getByLabel("Path", { exact: true }).fill(path);
  await dialog.getByLabel("Path", { exact: true }).press("Enter");
}

async function passed(dialog: Locator, fullName = ADMIN) {
  await expect(dialog.getByText(`Git checkout of ${fullName}`, { exact: true })).toBeVisible({ timeout: 15000 });
  await expect(dialog.getByText("on main · origin matches", { exact: true })).toBeVisible();
}

async function pickRepository(dialog: Locator, fullName: string) {
  await dialog.getByRole("option", { name: `${fullName} ${fullName === SITE ? "Public" : "Private"} · main`, exact: true }).click();
}

function answerCheckout(request: TestRepositoryCheckout): TestRepositoryCheckoutResult {
  const result: TestRepositoryCheckoutResult = {
    status: "completed",
    resolved_path: request.path.startsWith("~/") ? `/home/e2e/${request.path.slice(2)}` : request.path,
    passed: false, failures: [], version_files: [], last_tag: "", install_commands: [],
  };
  if (request.path.endsWith("/notes")) result.failures = [{ code: "not_git", detail: result.resolved_path }];
  else if (request.path.endsWith("/feature-login")) result.failures = [{ code: "wrong_branch", detail: "feature/login" }];
  else if (request.path.endsWith("/wrong-origin")) result.failures = [{ code: "origin_mismatch", detail: "git@github.com:appboypov/rebl.git" }];
  else if (request.full_name === SITE && (request.path === "~/Repos/rebl-site" || request.path.endsWith("/clone-fails")) && !request.clone) {
    result.failures = [{ code: "not_found", detail: `stat ${result.resolved_path}: no such file or directory` }];
  } else if (request.clone && request.path.endsWith("/clone-fails")) {
    result.failures = [{ code: "clone_failed", detail: "Could not resolve host github.com" }];
  } else {
    result.passed = true;
    result.version_files = request.full_name === WIDGETS ? ["pubspec.yaml", "package.json"] : request.full_name === SITE ? [] : ["package.json"];
    result.last_tag = request.full_name === WIDGETS ? "" : request.full_name === SITE ? "v2.1.0" : "v0.4.2";
  }
  return result;
}

function answerDirectory(request: TestDirectoryListing): TestDirectoryListingResult {
  // The real daemon expands the typed ~/Code path and returns absolute entry paths.
  if (request.path === "~/Code" || request.path === "/home/e2e/Code") {
    return { status: "completed", path: "/home/e2e/Code", entries: [{ name: "rebl-admin", path: "/home/e2e/Code/rebl-admin", kind: "folder", hidden: false }] };
  }
  if (request.path !== "") throw new Error(`Unexpected folder picker path: ${request.path}`);
  return { status: "completed", path: "", entries: [{ name: "~/Code", path: "/home/e2e/Code", kind: "folder", hidden: false }] };
}

test.describe("Repositories of a project / UJ-2", () => {
  let github: FakeGitHub;
  let api: TestApiClient | undefined;
  let suffix: string;

  test.beforeAll(async () => { github = await startGitHubFake(); });
  test.afterAll(async () => { await stopGitHubFake(github); });
  test.beforeEach(async () => {
    suffix = `${Date.now().toString(36)}-${randomUUID().slice(0, 8)}`;
    github.repositories.splice(0, github.repositories.length, ...[ADMIN, SITE, TAKEN].map((name, index) => githubRepository(name, 910001 + index, name !== SITE)));
    api = await createTestApi();
    await api.seedGitHubInstallation(Date.now());
  });
  test.afterEach(async () => { await api?.cleanup(); });

  function addVisibleRepository(fullName: string) {
    github.repositories.push(githubRepository(fullName, 910001 + github.repositories.length, fullName !== SITE));
  }

  async function setupProject(page: Page, client: TestApiClient) {
    const organisationName = `Rebl Records ${suffix}`;
    const projectName = `Rebl ${suffix}`;
    const organisation = await client.createOrganisation(organisationName, `rebl-${suffix}`);
    const project = await client.createProject(projectName, { organisation_id: organisation.id });
    const runtimeName = `Brian's Mac ${suffix}`;
    const runtime = await client.registerRepositoryRuntime(runtimeName);
    const slug = await loginAsDefault(page);
    const projectUrl = `/${slug}/projects/${project.id}`;
    await page.goto(projectUrl, { waitUntil: "domcontentloaded" });
    await waitForPageText(page, projectName);
    return { organisation, organisationName, project, projectName, runtime, runtimeName, slug, projectUrl };
  }

  test("App repository list, searching, taken repositories and giving access", async ({ page }) => {
    if (!api) throw new Error("Test API setup did not finish");
    const client = api;
    const context = await setupProject(page, client);
    await client.startRepositoryCheckoutAnswerer(context.runtime.id, answerCheckout);
    const photoPrankName = `Photo Prank ${suffix}`;
    const photoPrank = await client.createProject(photoPrankName);
    const check = await client.checkRepositoryCheckout(context.runtime.id, TAKEN, `/home/e2e/${suffix}/photoprank`);
    await client.createRepository(photoPrank.id, TAKEN, check);
    // Photo Prank and its repository exist before Brian opens Rebl's page.
    await page.reload({ waitUntil: "domcontentloaded" });
    await waitForPageText(page, context.projectName);

    // add-repository: The list; Nothing picked; A repository in another project.
    const add = await openAdd(page, context.projectName);
    await expect(add.getByRole("button", { name: "Add repository", exact: true })).toBeDisabled();
    await expect(add.getByRole("option", { name: `${ADMIN} Private · main`, exact: true })).toBeVisible();
    await expect(add.getByRole("option", { name: `${SITE} Public · main`, exact: true })).toBeVisible();
    await expect(add.getByLabel(/repository URL/i)).toHaveCount(0);
    await expect(add.getByRole("textbox")).toHaveCount(2);
    await expect(add.getByPlaceholder(/github\.com\/owner\/repo/i)).toHaveCount(0);
    const taken = add.getByRole("option", { name: `${TAKEN} In ${photoPrankName}`, exact: true });
    await expect(taken).toHaveAttribute("aria-disabled", "true");
    await taken.dispatchEvent("click");
    await expect(taken).toHaveAttribute("aria-selected", "false");
    await expect(add.getByRole("button", { name: "Add repository", exact: true })).toBeDisabled();

    // add-repository: Searching; No match.
    await add.getByLabel("Search repositories").fill("rebl");
    await expect(add.getByRole("option", { name: new RegExp(ADMIN) })).toBeVisible();
    await expect(add.getByRole("option", { name: new RegExp(TAKEN) })).toHaveCount(0);
    await add.getByLabel("Search repositories").fill("zzz");
    await expect(add.getByText("No repository matches “zzz”", { exact: true })).toBeVisible();
    await add.getByLabel("Search repositories").fill("");

    // add-repository: Giving access. Capture the button's external target without opening GitHub.
    await expect(add.getByRole("option", { name: new RegExp(WIDGETS) })).toHaveCount(0);
    await page.evaluate(() => {
      window.open = (url) => { document.body.dataset.e2eExternalUrl = String(url); return null; };
    });
    await add.getByRole("button", { name: "Missing one? Give Multica access on GitHub", exact: true }).click();
    const target = await page.evaluate(() => document.body.dataset.e2eExternalUrl);
    if (!target) throw new Error("The GitHub access button did not open an external target");
    const installationUrl = new URL(target);
    expect(installationUrl.origin + installationUrl.pathname).toBe("https://github.com/apps/multica-e2e/installations/new");
    expect(installationUrl.searchParams.get("state")).toBeTruthy();
    await add.getByRole("button", { name: "Cancel", exact: true }).click();
    addVisibleRepository(WIDGETS);
    const reopened = await openAdd(page, context.projectName);
    await expect(reopened.getByRole("option", { name: `${WIDGETS} Private · main`, exact: true })).toBeVisible();
  });

  test("Checkout checks, offline runtime and choosing another folder", async ({ page }) => {
    if (!api) throw new Error("Test API setup did not finish");
    const client = api;
    const context = await setupProject(page, client);
    await client.startRepositoryCheckoutAnswerer(context.runtime.id, answerCheckout, answerDirectory);
    const officeName = `Office iMac ${suffix}`;
    await client.registerRepositoryRuntime(officeName, "offline");
    // Reload so runtimeListOptions sees the second runtime even if the page prefetched it.
    await page.reload({ waitUntil: "domcontentloaded" });
    const add = await openAdd(page, context.projectName);
    await pickRepository(add, ADMIN);

    // add-repository: A valid checkout.
    await expect(add.getByLabel("Path", { exact: true })).toHaveValue("~/Repos/rebl-admin");
    await add.getByLabel("Runtime", { exact: true }).click();
    await page.getByRole("option", { name: `${context.runtimeName} · online`, exact: true }).click();
    await expect(add.getByLabel("Runtime", { exact: true })).toContainText(`${context.runtimeName} · online`);
    await passed(add);

    // add-repository: An offline runtime.
    await add.getByLabel("Runtime", { exact: true }).click();
    const offline = page.getByRole("option", { name: `${officeName} · offline`, exact: true });
    await expect(offline).toBeVisible();
    await expect(offline).toBeDisabled();
    await offline.dispatchEvent("click");
    await expect(add.getByLabel("Runtime", { exact: true })).toContainText(`${context.runtimeName} · online`);
    await page.keyboard.press("Escape");

    // add-repository: Another folder, through the real directory-listing heartbeat/result routes.
    await add.getByRole("button", { name: "Choose", exact: true }).click();
    const browse = page.getByRole("dialog", { name: "Browse runtime", exact: true });
    await expect(browse.getByRole("button", { name: "~/Code", exact: true })).toBeVisible({ timeout: 15000 });
    await browse.getByLabel("Path", { exact: true }).fill("~/Code");
    await browse.getByLabel("Path", { exact: true }).press("Enter");
    await browse.getByRole("checkbox", { name: "rebl-admin", exact: true }).check();
    await browse.getByRole("button", { name: "Add", exact: true }).click();
    await expect(browse).toHaveCount(0);
    await expect(add.getByLabel("Path", { exact: true })).toHaveValue("/home/e2e/Code/rebl-admin");
    await passed(add);

    // add-repository: Origin is another repository, shown as its owner/name, and its fix.
    await commitPath(add, `/home/e2e/${suffix}/wrong-origin`);
    await expect(add.getByText(`The folder's origin is appboypov/rebl, not ${ADMIN}`, { exact: true })).toBeVisible({ timeout: 15000 });
    await expect(add.getByText(`Choose your checkout of ${ADMIN}.`, { exact: true })).toBeVisible();
    await expect(add.getByRole("button", { name: "Add repository", exact: true })).toBeDisabled();

    // add-repository: Not on the default branch; A failed check.
    await commitPath(add, `/home/e2e/${suffix}/feature-login`);
    await expect(add.getByText("The folder is on feature/login, not main", { exact: true })).toBeVisible({ timeout: 15000 });
    await expect(add.getByText("Switch it to main, then check again.", { exact: true })).toBeVisible();
    await expect(add.getByRole("button", { name: "Add repository", exact: true })).toBeDisabled();

    // add-repository: Not a git checkout.
    const notes = `/home/e2e/${suffix}/notes`;
    await commitPath(add, notes);
    await expect(add.getByText(`${notes} is not a git checkout`, { exact: true })).toBeVisible({ timeout: 15000 });
    await expect(add.getByText(`Choose the folder of a git checkout of ${ADMIN}, or a path without a folder to clone it there.`, { exact: true })).toBeVisible();
    await expect(add.getByRole("button", { name: "Add repository", exact: true })).toBeDisabled();
  });

  test("Clone checkout outcomes and discovered release policies", async ({ page }) => {
    if (!api) throw new Error("Test API setup did not finish");
    const client = api;
    addVisibleRepository(WIDGETS);
    const context = await setupProject(page, client);
    const cloneGate = Promise.withResolvers<void>();
    const cloneStarted = Promise.withResolvers<TestRepositoryCheckout>();
    await client.startRepositoryCheckoutAnswerer(context.runtime.id, async (request, signal) => {
      if (request.clone && request.path === "~/Repos/rebl-site") {
        cloneStarted.resolve(request);
        const releaseOnAbort = () => cloneGate.resolve();
        if (signal.aborted) cloneGate.resolve();
        signal.addEventListener("abort", releaseOnAbort, { once: true });
        try { await cloneGate.promise; } finally { signal.removeEventListener("abort", releaseOnAbort); }
      }
      return answerCheckout(request);
    });
    const add = await openAdd(page, context.projectName);

    // add-repository: Cloning. The gate makes the transient Cloning state deterministic.
    await pickRepository(add, SITE);
    await expect(add.getByText("No folder at ~/Repos/rebl-site", { exact: true })).toBeVisible({ timeout: 15000 });
    await expect(add.getByRole("button", { name: "Clone it there", exact: true })).toBeVisible();
    try {
      await add.getByRole("button", { name: "Clone it there", exact: true }).click();
      await expect(add.getByText(`Cloning ${SITE}`, { exact: true })).toBeVisible();
      const request = await cloneStarted.promise;
      expect(request.clone).toBe(true);
      expect(request.path).toBe("~/Repos/rebl-site");
    } finally { cloneGate.resolve(); }
    await passed(add, SITE);
    await expect(add.getByRole("button", { name: "Add repository", exact: true })).toBeEnabled();

    // add-repository: No version file.
    await expect(add.getByLabel("Releases", { exact: true })).toContainText(TAGS);
    await expect(add.getByText("Last tag v2.1.0", { exact: true })).toBeVisible();

    // add-repository: The clone fails, including its recovery text and disabled Add.
    const failingPath = `/home/e2e/${suffix}/clone-fails`;
    await commitPath(add, failingPath);
    await expect(add.getByText(`No folder at ${failingPath}`, { exact: true })).toBeVisible({ timeout: 15000 });
    await add.getByRole("button", { name: "Clone it there", exact: true }).click();
    await expect(add.getByText("The clone failed: Could not resolve host github.com", { exact: true })).toBeVisible({ timeout: 15000 });
    await expect(add.getByText("Make sure this machine can reach GitHub over SSH, then clone again.", { exact: true })).toBeVisible();
    await expect(add.getByRole("button", { name: "Add repository", exact: true })).toBeDisabled();

    // add-repository: Two version files; No tags.
    await pickRepository(add, WIDGETS);
    await passed(add, WIDGETS);
    const pubspec = "Version in pubspec.yaml · tags v{version}";
    await expect(add.getByLabel("Releases", { exact: true })).toContainText(pubspec);
    await expect(add.getByText("No tags yet", { exact: true })).toBeVisible();
    await add.getByLabel("Releases", { exact: true }).click();
    for (const policy of [pubspec, VERSION, TAGS, NONE]) await expect(page.getByRole("option", { name: policy, exact: true })).toBeVisible();
    await page.getByRole("option", { name: pubspec, exact: true }).click();

    // add-repository: Does not release.
    await add.getByLabel("Releases", { exact: true }).click();
    await page.getByRole("option", { name: NONE, exact: true }).click();
    await expect(add.getByText("Its worklanes plan no releases.", { exact: true })).toBeVisible();

    // add-repository: One version file.
    await pickRepository(add, ADMIN);
    await passed(add);
    await expect(add.getByLabel("Releases", { exact: true })).toContainText(VERSION);
    await expect(add.getByText("Last tag v0.4.2", { exact: true })).toBeVisible();
    await add.getByLabel("Releases", { exact: true }).click();
    for (const policy of [VERSION, TAGS, NONE]) await expect(page.getByRole("option", { name: policy, exact: true })).toBeVisible();
    await page.getByRole("option", { name: VERSION, exact: true }).click();
  });

  test("Add, edit, fold and remove a repository without stranding a closed issue's release", async ({ page }) => {
    test.setTimeout(180000);
    if (!api) throw new Error("Test API setup did not finish");
    const client = api;
    const context = await setupProject(page, client);
    await client.startRepositoryCheckoutAnswerer(context.runtime.id, answerCheckout);
    const checkoutPath = `/home/e2e/${suffix}/Repos/rebl-admin`;
    const movedPath = `/home/e2e/${suffix}/Code/rebl-admin`;
    const notes = `/home/e2e/${suffix}/notes`;

    // project-repositories: A project without repositories; A project's page.
    await expect(page.getByText(/^0 repositories ·/)).toBeVisible();
    await expect(page.getByRole("button", { name: /^Fold or unfold appboypov\// })).toHaveCount(0);
    await expect(page.getByText("Resources", { exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /Add GitHub link|Add local folder/i })).toHaveCount(0);
    const add = await openAdd(page, context.projectName);
    await pickRepository(add, ADMIN);
    await commitPath(add, checkoutPath);
    await passed(add);

    // add-repository: Adding appboypov/rebl-admin; project-repositories: Rebl with one repository.
    await expect(add.getByRole("button", { name: "Add repository", exact: true })).toBeEnabled();
    await add.getByRole("button", { name: "Add repository", exact: true }).click();
    await expect(add).toHaveCount(0);
    const toggle = page.getByRole("button", { name: `Fold or unfold ${ADMIN}`, exact: true });
    const card = page.locator("section").filter({ has: toggle });
    await expect(toggle).toHaveAttribute("aria-expanded", "true");
    await expect(card.getByTitle("Default branch main", { exact: true })).toBeVisible();
    await expect(card.getByRole("link", { name: /^main(?:\s|$)/ })).toBeVisible();
    await expect(card.getByText("Free", { exact: true })).toBeVisible();
    await expect(page.getByText(/^1 repository ·/)).toBeVisible();
    const repository = (await client.listRepositories()).find((item) => item.project_id === context.project.id);
    if (!repository) throw new Error("Added repository was not persisted");
    expect(repository.checkout_path).toBe(checkoutPath);
    expect(repository.version_policy).toBe("package.json");
    const main = (await client.listRepositoryWorklanes(repository.id)).find((lane) => lane.name === "main");
    if (!main) throw new Error("Added repository has no main worklane");
    expect(main.state).toBe("free");

    // project-repositories: A card folds.
    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await expect(card.getByRole("link", { name: /^main(?:\s|$)/ })).toHaveCount(0);
    await expect(toggle).toBeVisible();
    await toggle.click();

    // add-repository: A repository already in this project.
    const alreadyAdded = await openAdd(page, context.projectName);
    const inThisProject = alreadyAdded.getByRole("option", { name: `${ADMIN} In ${context.projectName}`, exact: true });
    await expect(inThisProject).toHaveAttribute("aria-disabled", "true");
    await inThisProject.dispatchEvent("click");
    await expect(inThisProject).toHaveAttribute("aria-selected", "false");
    await expect(alreadyAdded.getByRole("button", { name: "Add repository", exact: true })).toBeDisabled();
    await alreadyAdded.getByRole("button", { name: "Cancel", exact: true }).click();
    // add-repository: Adding appboypov/rebl-admin, the other project's picker.
    const otherName = `Photo Prank ${suffix}`;
    const other = await client.createProject(otherName);
    await page.goto(`/${context.slug}/projects/${other.id}`, { waitUntil: "domcontentloaded" });
    const otherPicker = await openAdd(page, otherName);
    await expect(otherPicker.getByRole("option", { name: `${ADMIN} In ${context.projectName}`, exact: true })).toHaveAttribute("aria-disabled", "true");
    await otherPicker.getByRole("button", { name: "Cancel", exact: true }).click();
    await page.goto(context.projectUrl, { waitUntil: "domcontentloaded" });

    // project-repositories: The repository is fixed; A failed check; A moved checkout; Stop releasing.
    await card.getByRole("button", { name: `Actions of ${ADMIN}`, exact: true }).click();
    await page.getByRole("menuitem", { name: "Edit repository", exact: true }).click();
    const edit = page.getByRole("dialog", { name: "Edit repository", exact: true });
    await expect(edit.getByText(ADMIN, { exact: true })).toBeVisible();
    await expect(edit.getByRole("listbox", { name: "Repository", exact: true })).toHaveCount(0);
    await expect(edit.getByLabel("Search repositories")).toHaveCount(0);
    await expect(edit.getByLabel("Path", { exact: true })).toHaveValue(checkoutPath);
    await commitPath(edit, notes);
    await expect(edit.getByText(`${notes} is not a git checkout`, { exact: true })).toBeVisible({ timeout: 15000 });
    await expect(edit.getByRole("button", { name: "Save", exact: true })).toBeDisabled();
    await commitPath(edit, movedPath);
    await passed(edit);
    await edit.getByLabel("Releases", { exact: true }).click();
    await page.getByRole("option", { name: NONE, exact: true }).click();
    await expect(edit.getByText("Its worklanes plan no releases.", { exact: true })).toBeVisible();
    await edit.getByRole("button", { name: "Save", exact: true }).click();
    await expect(edit).toHaveCount(0);
    const edited = (await client.listRepositories()).find((item) => item.id === repository.id);
    expect(edited?.checkout_path).toBe(movedPath);
    expect(edited?.version_policy).toBe("none");
    // Cards show neither field: reopening Edit is their visible persisted readback.
    await card.getByRole("button", { name: `Actions of ${ADMIN}`, exact: true }).click();
    await page.getByRole("menuitem", { name: "Edit repository", exact: true }).click();
    await expect(edit.getByLabel("Path", { exact: true })).toHaveValue(movedPath);
    await expect(edit.getByLabel("Releases", { exact: true })).toContainText(NONE);
    await edit.getByRole("button", { name: "Cancel", exact: true }).click();

    // project-repositories: On the organisation's page.
    await page.goto(`/${context.slug}/organisations/${context.organisation.id}`, { waitUntil: "domcontentloaded" });
    const fold = page.getByRole("button", { name: `Fold or unfold the repositories of ${context.projectName}`, exact: true });
    await expect(fold).toBeVisible({ timeout: 15000 });
    if (await fold.getAttribute("aria-expanded") !== "true") await fold.click();
    const repositoryRow = page.getByRole("row").filter({ has: page.getByText(ADMIN, { exact: true }) });
    await expect(repositoryRow).toHaveCount(1);
    await expect(repositoryRow.getByText("main", { exact: true })).toBeVisible();

    // project-repositories: Removing appboypov/rebl-admin; clearRepositoryIssueReleases regression.
    // Restore a releasing policy before planning the regression's release GIVEN.
    await client.updateRepository(repository.id, "package.json");
    const release = await client.planRepositoryRelease(repository.id, `Removal regression ${suffix}`);
    const issue: { id: string } = await client.createIssue(`Closed release issue ${suffix}`, { project_id: context.project.id, worklane_id: main.id, status: "done" });
    await client.addIssueToRelease(release.id, issue.id);
    expect((await client.getIssue(issue.id)).release_id).toBe(release.id);
    expect((await client.getIssue(issue.id)).status).toBe("done");
    await page.goto(context.projectUrl, { waitUntil: "domcontentloaded" });
    await card.getByRole("button", { name: `Actions of ${ADMIN}`, exact: true }).click();
    await page.getByRole("menuitem", { name: "Remove repository", exact: true }).click();
    await expect(toggle).toHaveCount(0);
    await expect(page.getByText(/^0 repositories ·/)).toBeVisible();
    expect((await client.getIssue(issue.id)).release_id).toBeNull();
    expect(await client.listRepositoryWorklanes(repository.id)).toEqual([]);
    const addAgain = await openAdd(page, context.projectName);
    await expect(addAgain.getByRole("option", { name: `${ADMIN} Private · main`, exact: true })).not.toHaveAttribute("aria-disabled", "true");
    await addAgain.getByRole("button", { name: "Cancel", exact: true }).click();
    await page.goto(`/${context.slug}/organisations`, { waitUntil: "domcontentloaded" });
    const orgRow = page.getByRole("row").filter({ has: page.getByText(context.organisationName, { exact: true }) });
    await expect(orgRow.getByText("1 project · 0 repositories", { exact: true })).toBeVisible();
    await page.goto(`/${context.slug}/organisations/${context.organisation.id}`, { waitUntil: "domcontentloaded" });
    await expect(page.getByText("0 repositories", { exact: true })).toBeVisible();
  });

  test("Three repository rows and organisation aggregation across three projects", async ({ page }) => {
    test.setTimeout(180000);
    if (!api) throw new Error("Test API setup did not finish");
    const client = api;
    for (const name of [...MADSPEC_REPOS, "appboypov/multica"]) addVisibleRepository(name);
    const organisationName = `De App Specialist ${suffix}`;
    const organisation = await client.createOrganisation(organisationName, `das-${suffix}`);
    const madSpecName = `MadSpec ${suffix}`;
    const madSpec = await client.createProject(madSpecName, { organisation_id: organisation.id });
    const multica = await client.createProject(`Multica ${suffix}`, { organisation_id: organisation.id });
    await client.createProject(`DAS HQ ${suffix}`, { organisation_id: organisation.id });
    const runtime = await client.registerRepositoryRuntime(`Brian's Mac ${suffix}`);
    await client.startRepositoryCheckoutAnswerer(runtime.id, answerCheckout);
    for (const fullName of MADSPEC_REPOS) {
      const check = await client.checkRepositoryCheckout(runtime.id, fullName, `/home/e2e/${suffix}/${fullName.split("/")[1]}`);
      await client.createRepository(madSpec.id, fullName, check);
    }
    const multicaCheck = await client.checkRepositoryCheckout(runtime.id, "appboypov/multica", `/home/e2e/${suffix}/multica`);
    await client.createRepository(multica.id, "appboypov/multica", multicaCheck);
    const slug = await loginAsDefault(page);

    // project-repositories: MadSpec's repositories on Projects, the exact three-repository example.
    await page.goto(`/${slug}/projects`, { waitUntil: "domcontentloaded" });
    const fold = page.getByRole("button", { name: `Fold or unfold the repositories of ${madSpecName}`, exact: true });
    // The fold shows once the repository list has loaded after the projects.
    await expect(fold).toBeVisible({ timeout: 15000 });
    if (await fold.getAttribute("aria-expanded") !== "true") await fold.click();
    const rows = page.getByRole("row").filter({ has: page.getByText(/^appboypov\/(madspec|plannotator|omp-anytype-inbox)$/, { exact: true }) });
    await expect(rows).toHaveCount(3);
    for (const fullName of MADSPEC_REPOS) {
      const row = rows.filter({ has: page.getByText(fullName, { exact: true }) });
      await expect(row.getByText(fullName, { exact: true })).toBeVisible();
      await expect(row.getByText("main", { exact: true })).toBeVisible();
    }

    // project-repositories: De App Specialist, the exact three-project/four-repository sum.
    await page.goto(`/${slug}/organisations`, { waitUntil: "domcontentloaded" });
    const orgRow = page.getByRole("row").filter({ has: page.getByText(organisationName, { exact: true }) });
    await expect(orgRow.getByText("3 projects · 4 repositories", { exact: true })).toBeVisible();
    await page.goto(`/${slug}/organisations/${organisation.id}`, { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: organisationName, level: 1 })).toBeVisible();
    await expect(page.getByText("3 projects", { exact: true })).toBeVisible();
    await expect(page.getByText("4 repositories", { exact: true })).toBeVisible();

    // project-repositories: A repository added, a four-to-five sum across the same projects.
    const extraCheck = await client.checkRepositoryCheckout(runtime.id, ADMIN, `/home/e2e/${suffix}/rebl-admin`);
    await client.createRepository(multica.id, ADMIN, extraCheck);
    await page.goto(`/${slug}/organisations`, { waitUntil: "domcontentloaded" });
    await expect(orgRow.getByText("3 projects · 5 repositories", { exact: true })).toBeVisible();
    await page.goto(`/${slug}/organisations/${organisation.id}`, { waitUntil: "domcontentloaded" });
    await expect(page.getByText("3 projects", { exact: true })).toBeVisible();
    await expect(page.getByText("5 repositories", { exact: true })).toBeVisible();
  });
});
