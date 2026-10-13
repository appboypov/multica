import { test as base, expect as baseExpect, type Page, type Response } from "@playwright/test";
import { mkdtemp, mkdir, realpath, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { TestApiClient } from "./fixtures";

const expect = baseExpect.configure({ timeout: 30000 });

interface Agent {
  id: string;
  name: string;
  description: string;
  instructions: string;
  instruction_selection: string[];
  runtime_id: string;
}

interface Runtime {
  id: string;
  owner_id: string;
  provider: string;
  status: string;
}

// The managed dev daemon belongs to MULTICA_DEV_EMAIL (dev@localhost by
// default). A fresh unrelated E2E account cannot browse its private runtime.
class SelectionApiClient extends TestApiClient {
  fixtureWorkspaceId = "";
  readonly agentIds = new Set<string>();

  async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const response = await fetch(
      `${process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`}${path}`,
      {
        ...init,
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${this.getToken()}`,
          "X-Workspace-ID": this.fixtureWorkspaceId,
        },
      },
    );
    if (!response.ok) {
      throw new Error(`${init.method ?? "GET"} ${path}: ${response.status} ${await response.text()}`);
    }
    return response.json();
  }

  async seedAgent(runtimeId: string, paths: string[], instructions = "{selection}") {
    const agent = await this.request<Agent>("/api/agents", {
      method: "POST",
      body: JSON.stringify({
        name: `E2E selection ${crypto.randomUUID()}`,
        runtime_id: runtimeId,
        instructions,
        instruction_selection: paths,
      }),
    });
    this.agentIds.add(agent.id);
    return agent;
  }

  async cleanup() {
    try {
      // Agents expose archive, not DELETE. Only archive this test's creations.
      for (const id of this.agentIds) {
        await this.request(`/api/agents/${id}/archive`, { method: "POST" });
      }
    } finally {
      await super.cleanup();
    }
  }
}

interface SelectionFixture {
  api: SelectionApiClient;
  slug: string;
  runtime: Runtime;
  skills: string;
  notes: string;
  folder: string;
  first: string;
  second: string;
}

const test = base.extend<{ selection: SelectionFixture }>({
  selection: async ({ page }, use) => {
    const api = new SelectionApiClient();
    const identity = await api.login(process.env.MULTICA_DEV_EMAIL || "dev@localhost", "");
    const workspace = await api.ensureWorkspace();
    api.fixtureWorkspaceId = workspace.id;
    await api.markUserOnboarded();
    const runtimes = await api.request<Runtime[]>(`/api/runtimes?workspace_id=${workspace.id}`);
    const runtime = runtimes.find(
      (item) => item.provider === "omp" && item.owner_id === identity.user.id && item.status === "online",
    );
    if (!runtime) throw new Error("The signed-in dev user needs an online owned omp runtime; run make up C=daemon");

    // Keep the runtime fixture outside a home folder so chip text is not
    // abbreviated to "~"; canonicalize macOS's /tmp -> /private/tmp alias.
    const tree = await realpath(await mkdtemp("/tmp/multica-selection-e2e-"));
    const skills = join(tree, "s");
    const notes = join(tree, "n");
    const folder = join(skills, "as-ux-designer");
    const first = join(notes, "b.md");
    const second = join(notes, "c.md");
    await mkdir(folder, { recursive: true });
    await mkdir(notes);
    await writeFile(first, "Existing selected instructions.");
    await writeFile(second, "New selected instructions.");
    await writeFile(join(folder, "SKILL.md"), "# E2E UX fixture");
    await page.addInitScript((token) => {
      localStorage.setItem("multica_token", token);
      localStorage.setItem("multica:chat:isOpen", "false");
    }, api.getToken()!);
    try {
      await use({ api, slug: workspace.slug, runtime, skills, notes, folder, first, second });
    } finally {
      try {
        await api.cleanup();
      } finally {
        await rm(tree, { recursive: true, force: true });
      }
    }
  },
});

test.use({
  viewport: { width: 1280, height: 1200 },
  screenshot: "only-on-failure",
  trace: "retain-on-failure",
});

async function openInstructions(page: Page, selection: SelectionFixture, agent: Agent) {
  await page.goto(`/${selection.slug}/agents/${agent.id}`, { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("heading", { name: agent.name, exact: true })).toBeVisible();
  // Jev's chips route: Capabilities defaults to its Instructions subsection.
  await page.getByRole("tab", { name: "Capabilities", exact: true }).click();
  await expect(page.getByRole("textbox", { name: "System prompt", exact: true })).toBeVisible();
}

async function openPath(page: Page, path: string, entry: string) {
  const dialog = page.getByRole("dialog", { name: "Browse runtime", exact: true });
  const input = dialog.getByRole("textbox", { name: "Path", exact: true });
  await input.fill(path);
  // Jev clicks "Open Path", which only focuses the textbox. The actual form
  // submits paths on Enter; replay that user action rather than the bad route.
  await input.press("Enter");
  await expect(dialog.getByRole("checkbox", { name: entry, exact: true })).toBeVisible({ timeout: 30000 });
  return dialog;
}

function chips(page: Page) {
  return page.getByRole("list", { name: "Selected files and folders", exact: true }).getByRole("listitem");
}

async function selectCreateRuntime(page: Page) {
  const execution = page.getByRole("region", { name: "Execution", exact: true });
  await execution.getByRole("button").first().click();
  await page.getByRole("dialog").getByRole("button", { name: /^Oh-My-Pi / }).click();
}

async function createdAgent(response: Response, api: SelectionApiClient) {
  const body = await response.json();
  if (body.id) api.agentIds.add(body.id);
  expect(response.ok(), JSON.stringify(body)).toBeTruthy();
  return body as Agent;
}

test("One instructions field everywhere / Chips in the instructions tab", async ({ page, selection }) => {
  const agent = await selection.api.seedAgent(selection.runtime.id, [selection.first, selection.folder]);
  await openInstructions(page, selection, agent);
  // THEN (hand-written, kept on re-find)
  await expect(chips(page)).toHaveText([selection.first, selection.folder]);
  // END THEN
});

test("One instructions field everywhere / Removing a chip", async ({ page, selection }) => {
  const agent = await selection.api.seedAgent(selection.runtime.id, [selection.first, selection.second]);
  await openInstructions(page, selection, agent);
  await page.getByRole("button", { name: `Remove ${selection.first}`, exact: true }).click();
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect.poll(async () => (
    await selection.api.request<Agent>(`/api/agents/${agent.id}`)
  ).instruction_selection).toEqual([selection.second]);
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.getByRole("tab", { name: "Capabilities", exact: true }).click();
  await expect(chips(page)).toHaveText([selection.second]);
});

test("One instructions field everywhere / Selecting while creating an agent", async ({ page, selection }) => {
  // CreateAgentDialog has no web/desktop caller; the reachable create flow is
  // the manual create panel. No test-only route is added to mount dead UI.
  await page.goto(`/${selection.slug}/agents/new/manual`, { waitUntil: "domcontentloaded" });
  const name = `E2E selected create ${crypto.randomUUID()}`;
  await page.getByRole("textbox", { name: "Name", exact: true }).fill(name);
  await selectCreateRuntime(page);
  await page.getByRole("button", { name: "Browse", exact: true }).click();
  const browser = await openPath(page, selection.skills, "as-ux-designer");
  await browser.getByRole("checkbox", { name: "as-ux-designer", exact: true }).check();
  await browser.getByRole("button", { name: "Add", exact: true }).click();
  await expect(chips(page)).toHaveText([selection.folder]);
  const response = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/agents" && r.request().method() === "POST");
  await page.getByRole("button", { name: "Create & open agent", exact: true }).click();
  const agent = await createdAgent(await response, selection.api);
  const persisted = await selection.api.request<Agent>(`/api/agents/${agent.id}`);
  expect(persisted.instruction_selection).toEqual([selection.folder]);
  expect(persisted.runtime_id).toBe(selection.runtime.id);
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  await page.getByRole("tab", { name: "Capabilities", exact: true }).click();
  await expect(chips(page)).toHaveText([selection.folder]);
});

test("The first selection adds the placeholder / A first selection into typed text", async ({ page, selection }) => {
  const agent = await selection.api.seedAgent(selection.runtime.id, [], "Follow this.");
  await openInstructions(page, selection, agent);
  const text = page.getByRole("textbox", { name: "System prompt", exact: true });
  await text.fill("Follow this.");
  await expect.poll(() => text.evaluate((element: HTMLTextAreaElement) => element.selectionStart)).toBe("Follow this.".length);
  await page.getByRole("button", { name: "Browse", exact: true }).click();
  const browser = await openPath(page, selection.skills, "as-ux-designer");
  await browser.getByRole("checkbox", { name: "as-ux-designer", exact: true }).check();
  await browser.getByRole("button", { name: "Add", exact: true }).click();
  await expect(text).toHaveValue("Follow this.{selection}");
  await expect(chips(page)).toHaveText([selection.folder]);
});

test("The browser shows the runtime machine's folders / A typed path", async ({ page, selection }) => {
  const agent = await selection.api.seedAgent(selection.runtime.id, [], "");
  await openInstructions(page, selection, agent);
  await page.getByRole("button", { name: "Browse", exact: true }).click();
  const browser = await openPath(page, selection.notes, "b.md");
  await expect(browser.getByRole("checkbox")).toHaveCount(2);
  await expect(browser.getByRole("checkbox", { name: "b.md", exact: true })).toBeVisible();
  await expect(browser.getByRole("checkbox", { name: "c.md", exact: true })).toBeVisible();
  await expect(browser.getByText("b.md", { exact: true })).toHaveAttribute("title", selection.first);
  await expect(browser.getByText("c.md", { exact: true })).toHaveAttribute("title", selection.second);
});

test("The browser selects several paths at once / Two picks from different folders", async ({ page, selection }) => {
  const agent = await selection.api.seedAgent(selection.runtime.id, [selection.first]);
  await openInstructions(page, selection, agent);
  await page.getByRole("button", { name: "Browse", exact: true }).click();
  const browser = await openPath(page, selection.skills, "as-ux-designer");
  await browser.getByRole("checkbox", { name: "as-ux-designer", exact: true }).check();
  await openPath(page, selection.notes, "c.md");
  await browser.getByRole("checkbox", { name: "c.md", exact: true }).check();
  await browser.getByRole("button", { name: "Add", exact: true }).click();
  await expect(chips(page)).toHaveText([selection.first, selection.folder, selection.second]);
  await expect(page.getByRole("textbox", { name: "System prompt", exact: true })).toHaveValue("{selection}");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect.poll(async () => (
    await selection.api.request<Agent>(`/api/agents/${agent.id}`)
  ).instruction_selection).toEqual([selection.first, selection.folder, selection.second]);
});

test("The description inputs warn above 255 characters / A long description in the form", async ({ page, selection }) => {
  await page.goto(`/${selection.slug}/agents/new/manual`, { waitUntil: "domcontentloaded" });
  const name = `E2E long description ${crypto.randomUUID()}`;
  const description = "d".repeat(300);
  await page.getByRole("textbox", { name: "Name", exact: true }).fill(name);
  await page.getByRole("textbox", { name: "Description", exact: true }).fill(description);
  await expect(page.getByRole("textbox", { name: "Description", exact: true })).toHaveValue(description);
  const counter = page.getByRole("region", { name: "Identity", exact: true }).getByText(/^300 \/ 255/);
  await expect(counter).toBeVisible();
  await expect(counter).toHaveClass(/text-warning/);
  await expect(counter).toContainText("45");
  const response = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/agents" && r.request().method() === "POST");
  await page.getByRole("button", { name: "Create & open agent", exact: true }).click();
  const agent = await createdAgent(await response, selection.api);
  expect((await selection.api.request<Agent>(`/api/agents/${agent.id}`)).description).toBe(description);
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  await expect(page.getByText(description, { exact: true })).toBeVisible();
});
