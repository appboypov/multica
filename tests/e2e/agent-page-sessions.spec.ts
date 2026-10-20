import { test as base, expect as baseExpect, type Page } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "../../e2e/fixtures";
import { setTimeout as sleep } from "node:timers/promises";

const expect = baseExpect.configure({ timeout: 30000 });

const folder = "/Users/brian/Worktrees/de-app-specialist/projects/agent-offices/ongoing-development/registered-terminal-sessions/mitchel-office";
const runFolder = "/work/agents/mitchel";
const repository = "appboypov/agents";

interface SessionsFixture {
  slug: string;
  mitchel: string;
  guus: string;
  terminal: string;
  run: string;
  issue: string;
  endSession: () => Promise<void>;
}

const test = base.extend<{ sessions: SessionsFixture }>({
  sessions: async ({ page }, use) => {
    const api = new TestApiClient();
    const suffix = crypto.randomUUID();
    const identity = await api.login(`e2e-sessions-${suffix}@multica.ai`, "Sessions Tester");
    const workspace = await api.ensureWorkspace("Sessions acceptance", `e2e-sessions-${suffix}`);
    await api.markUserOnboarded();
    const db = new pg.Client(process.env.DATABASE_URL);
    await db.connect();
    const request = async (path: string, method: string, body?: unknown) => {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`}${path}`, {
        method,
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${api.getToken()}`,
          "X-Workspace-ID": workspace.id,
        },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      if (!response.ok) throw new Error(`${method} ${path}: ${response.status} ${await response.text()}`);
      return response.status === 204 ? undefined : response.json();
    };
    let stopRuntime = async () => {};
    try {
      await db.query("UPDATE workspace SET issue_prefix = 'WORK', issue_counter = 1057 WHERE id = $1", [workspace.id]);
      const daemon = `sessions-${suffix}`;
      const runtime = (await db.query<{ id: string }>(
        `INSERT INTO agent_runtime (workspace_id, daemon_id, name, runtime_mode, provider, status, owner_id, last_seen_at)
         VALUES ($1, $2, 'Mitchel test runtime', 'local', 'omp', 'online', $3, now()) RETURNING id`,
        [workspace.id, daemon, identity.user.id],
      )).rows[0]!.id;
      // Keep the fixture daemon online while the browser opens client routes.
      let serving = true;
      const runtimeLoop = (async () => {
        while (serving) {
          await request("/api/daemon/heartbeat", "POST", { runtime_id: runtime });
          await sleep(1000);
        }
      })();
      stopRuntime = async () => { serving = false; await runtimeLoop; };
      const mitchel = (await request("/api/agents", "POST", { name: "Mitchel", runtime_id: runtime })).id as string;
      const guus = (await request("/api/agents", "POST", { name: "Guus", runtime_id: runtime })).id as string;
      const project = await api.createProject("Agent offices");
      const repo = (await db.query<{ id: string }>(
        `INSERT INTO repository (workspace_id, project_id, github_repo_id, full_name, private, default_branch, version_policy, checkout_daemon_id, checkout_path)
         VALUES ($1, $2, 1057, $3, false, 'main', 'none', $4, '/work/agents') RETURNING id`,
        [workspace.id, project.id, repository, daemon],
      )).rows[0]!.id;
      const issue = (await db.query<{ id: string }>(
        `INSERT INTO issue (workspace_id, number, title, creator_type, creator_id, status)
         VALUES ($1, 1057, 'Mitchel agent office', 'member', $2, 'in_progress') RETURNING id`,
        [workspace.id, identity.user.id],
      )).rows[0]!.id;
      await db.query(
        `INSERT INTO issue_worktree (workspace_id, issue_id, repository_id, daemon_id, path, branch, state)
         VALUES ($1, $2, $3, $4, $5, 'work-1057', 'active')`,
        [workspace.id, issue, repo, daemon, runFolder],
      );
      const run = (await db.query<{ id: string }>(
        `INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, work_dir, dispatched_at, started_at)
         VALUES ($1, $2, $3, 'running', $4, now(), now()) RETURNING id`,
        [mitchel, runtime, issue, runFolder],
      )).rows[0]!.id;
      const terminal = (await request("/api/daemon/agent-sessions", "POST", {
        workspace_id: workspace.id, daemon_id: daemon, agent_id: mitchel,
        pid: 4242, process_started_at: new Date().toISOString(), herdr_pane_id: "w12:p0",
        work_dir: folder, repository, branch: "main", status: "idle",
      })).id as string;
      await page.addInitScript((token) => {
        localStorage.setItem("multica_token", token);
        localStorage.setItem("multica:chat:isOpen", "false");
      }, api.getToken()!);
      await use({
        slug: workspace.slug, mitchel, guus, terminal, run, issue,
        endSession: async () => {
          await request(`/api/daemon/agent-sessions/${terminal}?workspace_id=${workspace.id}&daemon_id=${daemon}`, "DELETE");
        },
      });
    } finally {
      await stopRuntime();
      // Remove only this test's workspace and its dependent fixtures.
      await db.query("DELETE FROM agent_task_queue WHERE agent_id IN (SELECT id FROM agent WHERE workspace_id = $1)", [workspace.id]);
      await db.query("DELETE FROM agent_session WHERE workspace_id = $1", [workspace.id]);
      await db.query("DELETE FROM agent WHERE workspace_id = $1", [workspace.id]);
      await api.deleteWorkspace();
      await db.end();
    }
  },
});

test.use({ viewport: { width: 1280, height: 1000 }, trace: "retain-on-failure" });
test.describe.configure({ timeout: 120000 });

async function openOverview(page: Page, sessions: SessionsFixture, agent: "mitchel" | "guus") {
  await page.goto(`/${sessions.slug}/agents/${sessions[agent]}`);
  await expect(page.getByRole("heading", { name: agent === "mitchel" ? "Mitchel" : "Guus", exact: true })).toBeVisible();
  await page.getByRole("tab", { name: "Overview", exact: true }).click();
}

test("The agent page shows its active sessions / Two sessions", async ({ page, sessions }, testInfo) => {
  await openOverview(page, sessions, "mitchel");
  const terminal = page.getByTestId(`session-${sessions.terminal}`);
  const run = page.getByTestId(`session-${sessions.run}`);
  await expect(terminal).toBeVisible();
  await expect(run).toBeVisible();
  await expect(page.locator('[data-testid^="session-"]')).toHaveCount(2);
  await expect(terminal.getByText("w12:p0", { exact: true })).toBeVisible();
  await expect(terminal.getByTitle(folder, { exact: true })).toHaveText(folder.replace("/Users/brian", "~"));
  await expect(terminal.getByText(repository, { exact: true })).toBeVisible();
  await expect(terminal.getByText("main", { exact: true })).toBeVisible();
  await expect(terminal.getByText("Idle", { exact: true })).toBeVisible();
  await expect(run.getByText("run on Mitchel test runtime", { exact: true })).toBeVisible();
  await expect(run.getByTitle(runFolder, { exact: true })).toHaveText(runFolder);
  await expect(run.getByText(repository, { exact: true })).toBeVisible();
  await expect(run.getByText("work-1057", { exact: true })).toBeVisible();
  await expect(run.getByText("Working", { exact: true })).toBeVisible();
  const sessionsHeading = page.getByRole("heading", { name: "Sessions", exact: true });
  const nowHeading = page.getByRole("heading", { name: "Now", exact: true });
  expect((await sessionsHeading.boundingBox())!.y).toBeLessThan((await nowHeading.boundingBox())!.y);

  const path = terminal.getByTitle(folder, { exact: true });
  const clipping = await path.evaluate((element) => {
    const suffix = element.querySelector("bdi")!.firstChild!;
    const range = document.createRange();
    range.setStart(suffix, suffix.textContent!.lastIndexOf("mitchel-office"));
    range.setEnd(suffix, suffix.textContent!.length);
    const tail = range.getBoundingClientRect();
    const box = element.getBoundingClientRect();
    return {
      overflow: element.scrollWidth > element.clientWidth,
      direction: getComputedStyle(element).direction,
      tailVisible: tail.left >= box.left && tail.right <= box.right + 1,
    };
  });
  expect(clipping).toEqual({ overflow: true, direction: "rtl", tailVisible: true });
  await expect(path).toHaveAttribute("title", folder);
  const screenshot = testInfo.outputPath("sessions-start-truncation.png");
  await page.screenshot({ path: screenshot });
  await testInfo.attach("Start-truncated folder with complete path title", { path: screenshot, contentType: "image/png" });

  await expect(run.getByRole("link", { name: "WORK-1057", exact: true })).toHaveAttribute("href", `/${sessions.slug}/issues/${sessions.issue}`);
  await run.getByRole("link", { name: "WORK-1057", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/${sessions.slug}/issues/WORK-1057(?:[/?#]|$)`));
  await expect(page.getByText("Mitchel agent office", { exact: true }).first()).toBeVisible();
});

test("The agent page shows its active sessions / No sessions", async ({ page, sessions }) => {
  await openOverview(page, sessions, "guus");
  await expect(page.getByText("No active sessions", { exact: true })).toBeVisible();
  await expect(page.locator('[data-testid^="session-"]')).toHaveCount(0);
});

test("The agent page shows its active sessions / A session ends", async ({ page, sessions }) => {
  const frames: string[] = [];
  page.on("websocket", (socket) => {
    if (new URL(socket.url()).pathname !== "/ws") return;
    socket.on("framereceived", ({ payload }) => frames.push(payload.toString()));
  });
  await openOverview(page, sessions, "mitchel");
  await expect(page.getByTestId(`session-${sessions.terminal}`)).toBeVisible();
  await expect(page.getByTestId(`session-${sessions.run}`)).toBeVisible();
  // Wait for the authenticated subscription before ending the registered pane.
  await expect.poll(() => frames.some((frame) => JSON.parse(frame).type === "auth_ack")).toBe(true);
  let navigations = 0;
  page.on("framenavigated", (frame) => { if (frame === page.mainFrame()) navigations++; });
  const beforeEnd = frames.length;
  await sessions.endSession();
  await expect.poll(() => frames.slice(beforeEnd).some((frame) => frame.includes("agent:sessions") && frame.includes(sessions.mitchel))).toBe(true);
  await expect(page.getByTestId(`session-${sessions.terminal}`)).toHaveCount(0);
  await expect(page.getByTestId(`session-${sessions.run}`)).toBeVisible();
  expect(navigations).toBe(0);
});
