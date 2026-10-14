/**
 * TestApiClient — lightweight API helper for E2E test data setup/teardown.
 *
 * Uses raw fetch so E2E tests have zero build-time coupling to the web app.
 */

import "./env";
import pg from "pg";
import { randomUUID } from "node:crypto";
import { setTimeout as delay } from "node:timers/promises";

// `||` (not `??`) so an empty `NEXT_PUBLIC_API_URL=` in .env still falls
// back to localhost. dotenv sets unset-vs-empty both as "" — treating them
// the same matches user intent.
const API_BASE = process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`;
const DATABASE_URL = process.env.DATABASE_URL ?? "postgres://multica:multica@localhost:5432/multica?sslmode=disable";

interface TestWorkspace {
  id: string;
  name: string;
  slug: string;
}

export type TestIssueStatus =
  | "backlog"
  | "todo"
  | "in_progress"
  | "in_review"
  | "done"
  | "blocked"
  | "cancelled";

export type TestIssuePriority = "urgent" | "high" | "medium" | "low" | "none";

export interface TestTableIssueSeed {
  title: string;
  status?: TestIssueStatus;
  priority?: TestIssuePriority;
  parentIssueId?: string | null;
  position?: number;
}

export interface TestTableIssue {
  id: string;
  title: string;
  status: TestIssueStatus;
  number: number;
}

export interface TestQuestion {
  id: string;
  prompt: string;
  status: string;
  asked_in_task_id: string;
  wake_available: boolean;
  unseen_answer_count: number;
  answers: {
    answerer_user_id: string;
    value: unknown;
    display_text: string;
    updated_at: string;
  }[];
}

export interface TestClaimedTask {
  id: string;
  agent_id: string;
  issue_id: string;
  runtime_id: string;
  dispatched_at: string;
  auth_token: string;
  question_answers?: {
    waker_name: string;
    answer_count: number;
    answers: {
      question_id: string;
      prompt: string;
      display_text: string;
      answerer_name: string;
      updated_at: string;
    }[];
  };
}

export interface TestRepository {
  id: string;
  project_id: string;
  full_name: string;
  default_branch: string;
  checkout_daemon_id: string;
  checkout_path: string;
  version_policy: string;
  /** "" means None, and null means not found yet. */
  install_command: string | null;
}

export interface TestWorklane {
  id: string;
  repository_id: string;
  name: string;
  folder: string | null;
  path: string | null;
  is_main: boolean;
  state: string;
}

export interface TestRepositoryCheckout {
  id: string;
  full_name: string;
  default_branch: string;
  path: string;
  clone: boolean;
}

export interface TestRepositoryCheckoutResult {
  status: "completed";
  resolved_path: string;
  passed: boolean;
  failures: { code: "not_git" | "wrong_branch" | "origin_mismatch" | "not_found" | "clone_failed"; detail?: string }[];
  version_files: string[];
  last_tag: string;
  install_commands: { file: string; command: string }[];
}

export interface TestRelease {
  id: string;
  repository_id: string;
  position: number;
  version: string;
  bump: "patch" | "minor" | "major" | null;
  title: string;
}

export interface TestDirectoryListing {
  id: string;
  path: string;
}

export interface TestDirectoryListingResult {
  status: "completed";
  path: string;
  entries: { name: string; path: string; kind: "folder" | "file"; hidden: boolean }[];
}

/** A worklane request the heartbeat hands the daemon (`pending_worklane_request`). */
export interface TestWorklaneRequest {
  id: string;
  kind: "create" | "remove" | "switch";
  full_name: string;
  default_branch: string;
  checkout_path: string;
  path: string;
  install_command?: string;
  from_step?: string;
  worklane_id?: string;
}

export interface TestWorklaneStep {
  key: "fetch" | "add_worktree" | "install" | "open";
  state: "pending" | "running" | "done" | "skipped" | "failed";
  detail: string;
  output: string;
  duration_ms: number;
}

/** What the daemon posts to a worklane request's progress route. */
export interface TestWorklaneProgress {
  steps?: TestWorklaneStep[];
  path?: string;
  outcome?: "removed" | "refused" | "failed";
  reason?: { code: string; detail: string };
}

export type TestWorklaneAnswer = (
  request: TestWorklaneRequest,
  report: (progress: TestWorklaneProgress) => Promise<void>,
  signal: AbortSignal,
) => Promise<void>;

export class TestApiClient {
  private token: string | null = null;
  private workspaceSlug: string | null = null;
  private workspaceId: string | null = null;
  private email: string | null = null;
  private createdIssueIds: string[] = [];
  private createdProjectIds: string[] = [];
  private createdOrganisationIds: string[] = [];
  private seededIssueIds: string[] = [];
  private createdRepositoryIds: string[] = [];
  private createdRuntimeIds: string[] = [];
  private createdInstallationIds: string[] = [];
  private checkoutAnswerers = new Set<() => Promise<void>>();

  async login(email: string, name: string) {
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      // Keep each E2E login isolated so previous test runs do not trip the
      // per-email send-code rate limit.
      await client.query("DELETE FROM verification_code WHERE email = $1", [email]);

      // Step 1: Send verification code
      const sendRes = await fetch(`${API_BASE}/auth/send-code`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      if (!sendRes.ok) {
        throw new Error(`send-code failed: ${sendRes.status}`);
      }

      // Step 2: Read code from database
      const result = await client.query(
        "SELECT code FROM verification_code WHERE email = $1 AND used = FALSE AND expires_at > now() ORDER BY created_at DESC LIMIT 1",
        [email],
      );
      if (result.rows.length === 0) {
        throw new Error(`No verification code found for ${email}`);
      }

      const configuredDevCode = process.env.MULTICA_DEV_VERIFICATION_CODE?.trim();
      const code = configuredDevCode || result.rows[0].code;

      // Step 3: Verify code to get JWT
      const verifyRes = await fetch(`${API_BASE}/auth/verify-code`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, code }),
      });
      if (!verifyRes.ok) {
        throw new Error(`verify-code failed: ${verifyRes.status}`);
      }
      const data = await verifyRes.json();

      this.token = data.token;
      this.email = email;

      // Update user name if needed
      if (name && data.user?.name !== name) {
        await this.authedFetch("/api/me", {
          method: "PATCH",
          body: JSON.stringify({ name }),
        });
      }

      await client.query("DELETE FROM verification_code WHERE email = $1", [email]);

      return data;
    } finally {
      await client.end();
    }
  }

  async getWorkspaces(): Promise<TestWorkspace[]> {
    const res = await this.authedFetch("/api/workspaces");
    return res.json();
  }

  setWorkspaceId(id: string) {
    this.workspaceId = id;
  }

  setWorkspaceSlug(slug: string) {
    this.workspaceSlug = slug;
  }

  async ensureWorkspace(name = "E2E Workspace", slug = "e2e-workspace") {
    const workspaces = await this.getWorkspaces();
    const workspace = workspaces.find((item) => item.slug === slug) ?? workspaces[0];
    if (workspace) {
      this.workspaceId = workspace.id;
      this.workspaceSlug = workspace.slug;
      return workspace;
    }

    const res = await this.authedFetch("/api/workspaces", {
      method: "POST",
      body: JSON.stringify({ name, slug }),
    });
    if (res.ok) {
      const created = (await res.json()) as TestWorkspace;
      this.workspaceId = created.id;
      this.workspaceSlug = created.slug;
      return created;
    }

    const refreshed = await this.getWorkspaces();
    const created = refreshed.find((item) => item.slug === slug) ?? refreshed[0];
    if (created) {
      this.workspaceId = created.id;
      this.workspaceSlug = created.slug;
      return created;
    }

    throw new Error(`Failed to ensure workspace ${slug}: ${res.status} ${res.statusText}`);
  }

  async markUserOnboarded() {
    if (!this.email) {
      throw new Error("Cannot mark E2E user onboarded before login");
    }

    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const result = await client.query(
        `
          UPDATE "user"
          SET
            onboarded_at = COALESCE(onboarded_at, now()),
            onboarding_questionnaire = COALESCE(onboarding_questionnaire, '{}'::jsonb)
              || '{"source":["friends_colleagues"],"source_other":null,"source_skipped":false}'::jsonb
          WHERE email = $1
        `,
        [this.email],
      );
      if (result.rowCount !== 1) {
        throw new Error(`Failed to mark E2E user onboarded: ${this.email}`);
      }
    } finally {
      await client.end();
    }
  }

  /** Create a project and register it for cleanup. */
  async createProject(title: string, opts?: Record<string, unknown>) {
    const res = await this.authedFetch("/api/projects", {
      method: "POST",
      body: JSON.stringify({ title, ...opts }),
    });
    if (!res.ok) {
      throw new Error(`create project failed: ${res.status} ${await res.text()}`);
    }
    const project = await res.json();
    this.createdProjectIds.push(project.id);
    return project as { id: string; title: string; status: string };
  }

  async listProjects() {
    const res = await this.authedFetch("/api/projects");
    if (!res.ok) {
      throw new Error(`list projects failed: ${res.status} ${await res.text()}`);
    }
    const body = await res.json();
    return body.projects as { id: string; title: string; organisation_id: string | null }[];
  }

  async updateProject(id: string, updates: Record<string, unknown>) {
    const res = await this.authedFetch(`/api/projects/${id}`, {
      method: "PUT",
      body: JSON.stringify(updates),
    });
    if (!res.ok) {
      throw new Error(`update project failed: ${res.status} ${await res.text()}`);
    }
    return res.json();
  }

  async deleteProject(id: string) {
    await this.authedFetch(`/api/projects/${id}`, { method: "DELETE" });
  }

  /**
   * Create an organisation and register it for cleanup. The API has no
   * delete, so cleanup removes it from the database.
   */
  async createOrganisation(name: string, key: string) {
    const res = await this.authedFetch("/api/organisations", {
      method: "POST",
      body: JSON.stringify({ name, key }),
    });
    if (!res.ok) {
      throw new Error(`create organisation failed: ${res.status} ${await res.text()}`);
    }
    const organisation = await res.json();
    this.createdOrganisationIds.push(organisation.id);
    return organisation as { id: string; name: string; key: string };
  }

  async createIssue(title: string, opts?: Record<string, unknown>) {
    const res = await this.authedFetch("/api/issues", {
      method: "POST",
      body: JSON.stringify({ title, ...opts }),
    });
    const issue = await res.json();
    this.createdIssueIds.push(issue.id);
    return issue;
  }

  /**
   * Insert a large, deterministic issue fixture in one transaction.
   *
   * Browser E2E coverage for cursor-backed Table views needs 1,000+ rows,
   * which would make setup itself dominate the test if every row went through
   * the HTTP create endpoint. These rows intentionally contain no dependent
   * records; cleanup deletes exactly the returned IDs from the isolated E2E
   * workspace.
   */
  async seedTableIssues(rows: TestTableIssueSeed[]): Promise<TestTableIssue[]> {
    if (rows.length === 0) return [];
    if (!this.workspaceId || !this.email) {
      throw new Error("Cannot seed table issues before login and workspace setup");
    }

    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      await client.query("BEGIN");
      const userResult = await client.query<{ id: string }>(
        `SELECT id FROM "user" WHERE email = $1`,
        [this.email],
      );
      const creatorId = userResult.rows[0]?.id;
      if (!creatorId) {
        throw new Error(`Cannot resolve E2E creator for ${this.email}`);
      }

      const counterResult = await client.query<{ issue_counter: number }>(
        `
          UPDATE workspace
          SET issue_counter = issue_counter + $2
          WHERE id = $1
          RETURNING issue_counter
        `,
        [this.workspaceId, rows.length],
      );
      const finalCounter = Number(counterResult.rows[0]?.issue_counter);
      if (!Number.isFinite(finalCounter)) {
        throw new Error(`Cannot reserve issue numbers for workspace ${this.workspaceId}`);
      }
      const firstNumber = finalCounter - rows.length + 1;

      const inserted = await client.query<TestTableIssue>(
        `
          INSERT INTO issue (
            workspace_id,
            title,
            status,
            priority,
            creator_type,
            creator_id,
            parent_issue_id,
            position,
            number
          )
          SELECT
            $1::uuid,
            fixture.title,
            fixture.status,
            fixture.priority,
            'member',
            $2::uuid,
            fixture.parent_issue_id,
            fixture.position,
            fixture.number
          FROM unnest(
            $3::text[],
            $4::text[],
            $5::text[],
            $6::uuid[],
            $7::double precision[],
            $8::integer[]
          ) WITH ORDINALITY AS fixture(
            title,
            status,
            priority,
            parent_issue_id,
            position,
            number,
            ordinal
          )
          ORDER BY fixture.ordinal
          RETURNING id, title, status, number
        `,
        [
          this.workspaceId,
          creatorId,
          rows.map((row) => row.title),
          rows.map((row) => row.status ?? "backlog"),
          rows.map((row) => row.priority ?? "none"),
          rows.map((row) => row.parentIssueId ?? null),
          rows.map((row, index) => row.position ?? index + 1),
          rows.map((_row, index) => firstNumber + index),
        ],
      );
      await client.query("COMMIT");
      this.seededIssueIds.push(...inserted.rows.map((row) => row.id));
      return inserted.rows;
    } catch (error) {
      await client.query("ROLLBACK");
      throw error;
    } finally {
      await client.end();
    }
  }

  async deleteIssue(id: string) {
    await this.authedFetch(`/api/issues/${id}`, { method: "DELETE" });
  }

  async createComment(issueId: string, content: string) {
    const res = await this.authedFetch(`/api/issues/${issueId}/comments`, {
      method: "POST",
      body: JSON.stringify({ content }),
    });
    if (!res.ok) {
      throw new Error(`create comment failed: ${res.status} ${await res.text()}`);
    }
    return (await res.json()) as { id: string };
  }

  /** Adds an existing user to this client's workspace and returns the member id. */
  async addMemberByEmail(email: string, role: "admin" | "member" = "member"): Promise<string> {
    if (!this.workspaceId) throw new Error("Cannot add a member before a workspace is selected");
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const result = await client.query(
        `INSERT INTO member (workspace_id, user_id, role)
         SELECT $1, id, $3 FROM "user" WHERE email = $2
         RETURNING id`,
        [this.workspaceId, email, role],
      );
      if (result.rowCount !== 1) throw new Error(`No user with email ${email}`);
      return result.rows[0].id as string;
    } finally {
      await client.end();
    }
  }

  async removeMember(memberId: string) {
    const res = await this.authedFetch(`/api/workspaces/${this.workspaceId}/members/${memberId}`, {
      method: "DELETE",
    });
    if (!res.ok) {
      throw new Error(`remove member failed: ${res.status} ${await res.text()}`);
    }
  }

  /** Deletes this client's workspace; only for workspaces a test created. */
  async deleteWorkspace() {
    if (!this.workspaceId) return;
    await this.authedFetch(`/api/workspaces/${this.workspaceId}`, { method: "DELETE" });
  }

  /** Server-side issue search, for comparing against what the UI shows. */
  async searchIssues(q: string, opts: { includeClosed?: boolean; limit?: number } = {}) {
    const params = new URLSearchParams({ q, limit: String(opts.limit ?? 20) });
    if (opts.includeClosed) params.set("include_closed", "true");
    const res = await this.authedFetch(`/api/issues/search?${params}`);
    if (!res.ok) {
      throw new Error(`search issues failed: ${res.status} ${await res.text()}`);
    }
    return (await res.json()) as { issues: { id: string; identifier: string; title: string }[] };
  }

  async updateIssue(id: string, updates: Record<string, unknown>) {
    const res = await this.authedFetch(`/api/issues/${id}`, {
      method: "PUT",
      body: JSON.stringify(updates),
    });
    if (!res.ok) {
      throw new Error(`update issue failed: ${res.status} ${await res.text()}`);
    }
    return res.json();
  }

  /** Ask through the real agent-only endpoint with a claimed run's credential. */
  async askQuestion(taskToken: string, input: Record<string, unknown>): Promise<{ ask_id: string; questions: TestQuestion[] }> {
    const res = await this.authedFetch("/api/questions", {
      method: "POST",
      body: JSON.stringify(input),
    }, taskToken);
    if (!res.ok) {
      throw new Error(`ask question failed: ${res.status} ${await res.text()}`);
    }
    return res.json();
  }

  /** The read endpoint used by `multica question`, optionally as the asking agent. */
  async getQuestion(id: string, taskToken?: string): Promise<TestQuestion> {
    const res = await this.authedFetch(`/api/questions/${id}`, undefined, taskToken);
    if (!res.ok) {
      throw new Error(`get question failed: ${res.status} ${await res.text()}`);
    }
    return res.json();
  }

  /** Claim a fixture runtime's work without starting a daemon or an agent executable. */
  async claimRuntimeTask(runtimeId: string): Promise<TestClaimedTask | null> {
    const res = await this.authedFetch(`/api/daemon/runtimes/${runtimeId}/tasks/claim`, {
      method: "POST",
      body: JSON.stringify({}),
    });
    if (!res.ok) {
      throw new Error(`claim task failed: ${res.status} ${await res.text()}`);
    }
    const result: { task: TestClaimedTask | null } = await res.json();
    return result.task;
  }

  async startClaimedTask(task: TestClaimedTask): Promise<{ id: string; status: string }> {
    const res = await this.authedFetch(`/api/daemon/tasks/${task.id}/start`, {
      method: "POST",
      body: JSON.stringify({ runtime_id: task.runtime_id, dispatched_at: task.dispatched_at }),
    });
    if (!res.ok) {
      throw new Error(`start task failed: ${res.status} ${await res.text()}`);
    }
    return res.json();
  }

  /** Bind a fake GitHub App installation only to this client's workspace. */
  async seedGitHubInstallation(installationId: number): Promise<void> {
    if (!this.workspaceId || !this.email) throw new Error("Login and select a workspace first");
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const result = await client.query<{ id: string }>(
        `INSERT INTO github_installation
          (workspace_id, installation_id, account_login, account_type, connected_by_id)
         SELECT $1, $2, 'appboypov', 'User', id FROM "user" WHERE email = $3
         RETURNING id`,
        [this.workspaceId, installationId, this.email],
      );
      const row = result.rows[0];
      if (!row) throw new Error("Could not seed GitHub installation for the E2E user");
      this.createdInstallationIds.push(row.id);
    } finally {
      await client.end();
    }
  }

  /**
   * User JWT registration makes this local runtime owned by the viewer.
   * Capabilities go in `X-Client-Capabilities`, as a daemon sends them.
   */
  async registerRepositoryRuntime(
    deviceName: string,
    status: "online" | "offline" = "online",
    capabilities: string[] = [],
  ): Promise<{ id: string; daemon_id: string }> {
    if (!this.workspaceId) throw new Error("Select a workspace before registering a runtime");
    const body = await this.repositoryRequest<{ runtimes: { id: string; daemon_id: string }[] }>(
      "/api/daemon/register",
      {
        method: "POST",
        headers: capabilities.length > 0 ? { "X-Client-Capabilities": capabilities.join(",") } : undefined,
        body: JSON.stringify({
          workspace_id: this.workspaceId,
          daemon_id: randomUUID(),
          device_name: deviceName,
          // Named as the daemon names its runtimes: `{provider} ({device})`.
          runtimes: [{ name: `Codex (${deviceName})`, type: "codex", status }],
        }),
      },
    );
    this.createdRuntimeIds.push(...body.runtimes.map((runtime) => runtime.id));
    const runtime = body.runtimes[0];
    if (!runtime) throw new Error("Daemon registration returned no runtime");
    return runtime;
  }

  /**
   * Poll the real daemon queue. No browser route interception replaces checkout
   * checks or worklane requests. A worklane answer posts its own progress.
   */
  async startRepositoryCheckoutAnswerer(
    runtimeId: string,
    answer: (
      request: TestRepositoryCheckout,
      signal: AbortSignal,
    ) => TestRepositoryCheckoutResult | Promise<TestRepositoryCheckoutResult>,
    answerDirectory?: (request: TestDirectoryListing) => TestDirectoryListingResult,
    answerWorklane?: TestWorklaneAnswer,
  ): Promise<() => Promise<void>> {
    const controller = new AbortController();
    const tick = async () => {
      const heartbeat = await this.repositoryRequest<{
        pending_repository_checkout?: TestRepositoryCheckout | null;
        pending_directory_listing?: TestDirectoryListing | null;
        pending_worklane_request?: TestWorklaneRequest | null;
      }>("/api/daemon/heartbeat", {
        method: "POST",
        body: JSON.stringify({ runtime_id: runtimeId }),
        signal: controller.signal,
      });
      const request = heartbeat.pending_repository_checkout;
      if (request) {
        await this.repositoryRequest<{ status: string }>(
          `/api/daemon/runtimes/${runtimeId}/repository-checkouts/${request.id}/result`,
          { method: "POST", body: JSON.stringify(await answer(request, controller.signal)), signal: controller.signal },
        );
      }
      const directory = heartbeat.pending_directory_listing;
      if (directory) {
        if (!answerDirectory) throw new Error(`Unexpected directory listing: ${directory.path}`);
        await this.repositoryRequest<{ status: string }>(
          `/api/daemon/runtimes/${runtimeId}/directory-listings/${directory.id}/result`,
          { method: "POST", body: JSON.stringify(answerDirectory(directory)), signal: controller.signal },
        );
      }
      const worklane = heartbeat.pending_worklane_request;
      if (worklane) {
        if (!answerWorklane) throw new Error(`Unexpected worklane request: ${worklane.kind} ${worklane.path}`);
        const report = async (progress: TestWorklaneProgress) => {
          await this.repositoryRequest<unknown>(
            `/api/daemon/runtimes/${runtimeId}/worklane-requests/${worklane.id}/progress`,
            { method: "POST", body: JSON.stringify(progress), signal: controller.signal },
          );
        };
        await answerWorklane(worklane, report, controller.signal);
      }
    };
    await tick();
    let failure: unknown;
    const running = (async () => {
      while (!controller.signal.aborted) {
        await delay(100, undefined, { signal: controller.signal });
        await tick();
      }
    })().catch((error: unknown) => {
      if (!controller.signal.aborted) failure = error;
    });
    const stop = async () => {
      controller.abort();
      await running;
      this.checkoutAnswerers.delete(stop);
      if (failure) throw failure;
    };
    this.checkoutAnswerers.add(stop);
    return stop;
  }

  async listRepositories(): Promise<TestRepository[]> {
    const body = await this.repositoryRequest<{ repositories: TestRepository[] }>("/api/repositories");
    return body.repositories;
  }

  async checkRepositoryCheckout(runtimeId: string, fullName: string, path: string): Promise<string> {
    const check = await this.repositoryRequest<{ id: string }>(
      `/api/runtimes/${runtimeId}/repository-checkouts`,
      { method: "POST", body: JSON.stringify({ full_name: fullName, default_branch: "main", path }) },
    );
    const deadline = Date.now() + 15000;
    while (Date.now() < deadline) {
      const result = await this.repositoryRequest<{ status: string; passed: boolean }>(
        `/api/runtimes/${runtimeId}/repository-checkouts/${check.id}`,
      );
      if (result.status === "completed") {
        if (!result.passed) throw new Error(`Checkout failed for ${path}`);
        return check.id;
      }
      if (result.status === "failed" || result.status === "timeout") {
        throw new Error(`Checkout ${result.status} for ${path}`);
      }
      await delay(100);
    }
    throw new Error(`Timed out waiting for checkout ${path}`);
  }

  async createRepository(
    projectId: string,
    fullName: string,
    checkId: string,
    installCommand = "",
  ): Promise<TestRepository> {
    const repository = await this.repositoryRequest<TestRepository>("/api/repositories", {
      method: "POST",
      body: JSON.stringify({
        project_id: projectId, full_name: fullName, checkout_check_id: checkId,
        version_policy: "package.json", install_command: installCommand,
      }),
    });
    this.createdRepositoryIds.push(repository.id);
    return repository;
  }

  async updateRepository(id: string, versionPolicy: "package.json" | "none"): Promise<TestRepository> {
    return this.repositoryRequest<TestRepository>(`/api/repositories/${id}`, {
      method: "PUT", body: JSON.stringify({ version_policy: versionPolicy }),
    });
  }

  async listRepositoryWorklanes(repositoryId: string): Promise<TestWorklane[]> {
    const body = await this.repositoryRequest<{ worklanes: TestWorklane[] }>("/api/worklanes");
    return body.worklanes.filter((worklane) => worklane.repository_id === repositoryId);
  }

  /**
   * Create a worklane through the real request flow: the runtime's answerer
   * must answer create requests. Returns the saved worklane's id.
   */
  async createWorklane(repositoryId: string, name: string, folder: string, daemonId: string): Promise<string> {
    const created = await this.repositoryRequest<{ id: string }>("/api/worklane-requests", {
      method: "POST",
      body: JSON.stringify({ repository_id: repositoryId, name, folder, daemon_id: daemonId }),
    });
    const deadline = Date.now() + 15000;
    while (Date.now() < deadline) {
      const request = await this.repositoryRequest<{ status: string; worklane_id: string | null; reason: unknown }>(
        `/api/worklane-requests/${created.id}`,
      );
      if (request.status === "completed" && request.worklane_id) return request.worklane_id;
      if (request.status !== "running") {
        throw new Error(`Worklane ${name} ${request.status}: ${JSON.stringify(request.reason)}`);
      }
      await delay(100);
    }
    throw new Error(`Timed out creating worklane ${name}`);
  }

  async updateOrganisationKey(id: string, key: string): Promise<void> {
    await this.repositoryRequest<unknown>(`/api/organisations/${id}`, {
      method: "PUT", body: JSON.stringify({ key }),
    });
  }

  /** A runtime goes offline or online as its daemon's heartbeat would set it. */
  async setRuntimeStatus(runtimeId: string, status: "online" | "offline"): Promise<void> {
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      await client.query("UPDATE agent_runtime SET status = $1 WHERE id = $2", [status, runtimeId]);
    } finally {
      await client.end();
    }
  }

  /** A repository added before repositories had an install command. */
  async clearRepositoryInstallCommand(repositoryId: string): Promise<void> {
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      await client.query("UPDATE repository SET install_command = NULL WHERE id = $1", [repositoryId]);
    } finally {
      await client.end();
    }
  }

  async planRepositoryRelease(repositoryId: string, title: string): Promise<{ id: string }> {
    const body = await this.repositoryRequest<{ releases: { id: string; title: string }[] }>(
      `/api/repositories/${repositoryId}/releases`,
      { method: "POST", body: JSON.stringify({ title, version: "0.4.3" }) },
    );
    const release = body.releases.find((item) => item.title === title);
    if (!release) throw new Error(`Planned release ${title} was not returned`);
    return release;
  }

  /** Plan a release on a repository's line: a bump, or a version set by hand. Returns the line. */
  async planRelease(
    repositoryId: string,
    release: { title: string; description?: string; bump?: "patch" | "minor" | "major"; version?: string },
  ): Promise<TestRelease[]> {
    const body = await this.repositoryRequest<{ releases: TestRelease[] }>(
      `/api/repositories/${repositoryId}/releases`,
      { method: "POST", body: JSON.stringify({ description: "", ...release }) },
    );
    return body.releases;
  }

  async removeRelease(id: string): Promise<TestRelease[]> {
    const body = await this.repositoryRequest<{ releases: TestRelease[] }>(`/api/releases/${id}`, {
      method: "DELETE",
    });
    return body.releases;
  }

  async listReleases(repositoryId: string): Promise<TestRelease[]> {
    const body = await this.repositoryRequest<{ releases: TestRelease[] }>("/api/releases");
    return body.releases.filter((release) => release.repository_id === repositoryId);
  }

  async addIssueToRelease(releaseId: string, issueId: string): Promise<void> {
    await this.repositoryRequest<unknown>(`/api/releases/${releaseId}/issues`, {
      method: "POST", body: JSON.stringify({ issue_ids: [issueId] }),
    });
  }

  async getIssue(id: string): Promise<{ id: string; status: string; release_id: string | null }> {
    return this.repositoryRequest(`/api/issues/${id}`);
  }

  private async repositoryRequest<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await this.authedFetch(path, {
      ...init,
      signal: init?.signal
        ? AbortSignal.any([init.signal, AbortSignal.timeout(10000)])
        : AbortSignal.timeout(10000),
    });
    if (!response.ok) {
      throw new Error(`${init?.method ?? "GET"} ${path}: ${response.status} ${await response.text()}`);
    }
    if (response.status === 204) return undefined as T;
    return response.json();
  }

  /** Clean up this client's rows, removing repositories through their API first. */
  async cleanup() {
    if (this.seededIssueIds.length > 0 && this.workspaceId) {
      const client = new pg.Client(DATABASE_URL);
      await client.connect();
      try {
        await client.query(
          `DELETE FROM issue WHERE workspace_id = $1 AND id = ANY($2::uuid[])`,
          [this.workspaceId, this.seededIssueIds],
        );
      } finally {
        await client.end();
      }
      this.seededIssueIds = [];
    }
    for (const id of this.createdIssueIds) {
      try {
        await this.deleteIssue(id);
      } catch {
        /* ignore — may already be deleted */
      }
    }
    this.createdIssueIds = [];
    const answererErrors: unknown[] = [];
    for (const stop of this.checkoutAnswerers) {
      try {
        await stop();
      } catch (error: unknown) {
        answererErrors.push(error);
      }
    }
    // Include repositories created in the UI, but never another spec's projects.
    const projectIds = new Set(this.createdProjectIds);
    const repositoryIds = new Set(this.createdRepositoryIds);
    if (projectIds.size > 0 || repositoryIds.size > 0) {
      for (const repository of await this.listRepositories()) {
        if (projectIds.has(repository.project_id)) repositoryIds.add(repository.id);
      }
      // Remove repository refuses while a worklane besides main exists, and a
      // worklane's own Remove needs a daemon: its rows go from the database.
      if (repositoryIds.size > 0 && this.workspaceId) {
        const client = new pg.Client(DATABASE_URL);
        await client.connect();
        try {
          await client.query(
            `DELETE FROM activity_log WHERE workspace_id = $1 AND worklane_id IN
              (SELECT id FROM worklane WHERE workspace_id = $1 AND repository_id = ANY($2::uuid[]) AND NOT is_main)`,
            [this.workspaceId, [...repositoryIds]],
          );
          await client.query(
            `DELETE FROM worklane WHERE workspace_id = $1 AND repository_id = ANY($2::uuid[]) AND NOT is_main`,
            [this.workspaceId, [...repositoryIds]],
          );
        } finally {
          await client.end();
        }
      }
      for (const id of repositoryIds) {
        const response = await this.authedFetch(`/api/repositories/${id}`, { method: "DELETE" });
        if (!response.ok && response.status !== 404) {
          throw new Error(`Cleanup repository ${id}: ${response.status} ${await response.text()}`);
        }
      }
      this.createdRepositoryIds = [];
    }
    for (const id of this.createdRuntimeIds) {
      const response = await this.authedFetch(`/api/runtimes/${id}`, { method: "DELETE" });
      if (!response.ok && response.status !== 404) {
        throw new Error(`Cleanup runtime ${id}: ${response.status} ${await response.text()}`);
      }
    }
    this.createdRuntimeIds = [];
    if (this.createdInstallationIds.length > 0 && this.workspaceId) {
      const client = new pg.Client(DATABASE_URL);
      await client.connect();
      try {
        await client.query(
          "DELETE FROM github_installation WHERE workspace_id = $1 AND id = ANY($2::uuid[])",
          [this.workspaceId, this.createdInstallationIds],
        );
        this.createdInstallationIds = [];
      } finally {
        await client.end();
      }
    }
    // Projects last: an issue delete leaves no project reference behind, and
    // dropping the project first would strand the issues in the list.
    for (const id of this.createdProjectIds) {
      try {
        await this.deleteProject(id);
      } catch {
        /* ignore — may already be deleted */
      }
    }
    this.createdProjectIds = [];
    // Organisations have no delete endpoint: projects made in the UI inside one
    // go through the API first, then the rows go straight from the database.
    if (this.createdOrganisationIds.length > 0 && this.workspaceId) {
      const organisationIds = new Set(this.createdOrganisationIds);
      for (const project of await this.listProjects()) {
        if (project.organisation_id && organisationIds.has(project.organisation_id)) {
          await this.deleteProject(project.id);
        }
      }
      const client = new pg.Client(DATABASE_URL);
      await client.connect();
      try {
        await client.query(
          `DELETE FROM organisation WHERE workspace_id = $1 AND id = ANY($2::uuid[])`,
          [this.workspaceId, this.createdOrganisationIds],
        );
      } finally {
        await client.end();
      }
      this.createdOrganisationIds = [];
    }
    if (answererErrors.length > 0) {
      throw new AggregateError(answererErrors, "Repository checkout answerer failed");
    }
  }

  getToken() {
    return this.token;
  }

  getEmail() {
    if (!this.email) {
      throw new Error("Test API client is not logged in");
    }
    return this.email;
  }

  private async authedFetch(path: string, init?: RequestInit, authToken = this.token) {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...((init?.headers as Record<string, string>) ?? {}),
    };
    if (authToken) headers["Authorization"] = `Bearer ${authToken}`;
    if (this.workspaceSlug) headers["X-Workspace-Slug"] = this.workspaceSlug;
    else if (this.workspaceId) headers["X-Workspace-ID"] = this.workspaceId;
    return fetch(`${API_BASE}${path}`, { ...init, headers });
  }
}
