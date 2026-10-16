import "./env";

import { expect, test as base, type Locator, type Page } from "@playwright/test";
import pg from "pg";
import { type TestApiClient, type TestClaimedTask, type TestQuestion } from "./fixtures";
import { createTestApi } from "./helpers";

type QuestionFixture = {
  api: TestApiClient;
  db: pg.Client;
  workspace: { id: string; slug: string };
  userId: string;
  agentId: string;
  runtimeId: string;
  issue: { id: string; identifier: string; title: string };
  askingTask: TestClaimedTask;
  ask: (input?: Record<string, unknown>) => Promise<TestQuestion>;
  taskIds: () => Promise<string[]>;
};

// Seed only the runtime and prior runs. Asking, answering, notification delivery,
// Wake and claim all use the real backend. No daemon or agent process is needed.
const test = base.extend<{ question: QuestionFixture }>({
  question: async ({ page }, use) => {
    const api = await createTestApi();
    const db = new pg.Client(process.env.DATABASE_URL);
    await db.connect();
    let agentId: string | undefined;
    let runtimeId: string | undefined;
    try {
      await api.login(api.getEmail(), "Brian");
      const workspace = (await api.getWorkspaces())[0]!;
      const user = await db.query<{ id: string }>(`SELECT id FROM "user" WHERE email = $1`, [api.getEmail()]);
      const userId = user.rows[0]!.id;
      const runtime = await db.query<{ id: string }>(
        `INSERT INTO agent_runtime (workspace_id, name, runtime_mode, provider, status, device_info, metadata, owner_id, last_seen_at)
         VALUES ($1, 'Question E2E runtime', 'cloud', 'codex', 'online', 'E2E fixture without executor', '{}'::jsonb, $2, now()) RETURNING id`,
        [workspace.id, userId],
      );
      runtimeId = runtime.rows[0]!.id;
      const agent = await db.query<{ id: string }>(
        `INSERT INTO agent (workspace_id, name, description, instructions, runtime_mode, runtime_config, runtime_id, visibility, permission_mode, max_concurrent_tasks, owner_id)
         VALUES ($1, 'Pieter', '', '', 'cloud', '{}'::jsonb, $2, 'private', 'private', 1, $3) RETURNING id`,
        [workspace.id, runtimeId, userId],
      );
      agentId = agent.rows[0]!.id;
      const issue = await api.createIssue("Decide invoice rounding for WORK-412", { status: "in_progress" });
      // Assignment through the API would itself start an unrelated run.
      await db.query(`UPDATE issue SET assignee_type = 'agent', assignee_id = $1 WHERE id = $2`, [agentId, issue.id]);
      await db.query(
        `INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, created_at, started_at, completed_at)
         SELECT $1, $2, $3, 'completed', now() - n * interval '1 hour', now() - n * interval '1 hour', now() - n * interval '1 hour' + interval '1 minute'
         FROM generate_series(1, 2) AS n`,
        [agentId, runtimeId, issue.id],
      );
      await db.query(
        `INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status) VALUES ($1, $2, $3, 'queued')`,
        [agentId, runtimeId, issue.id],
      );
      const askingTask = await api.claimRuntimeTask(runtimeId);
      if (!askingTask) throw new Error("Question fixture run was not claimable");
      expect(askingTask.agent_id).toBe(agentId);
      expect(askingTask.auth_token).toMatch(/^mat_/);
      expect(await api.startClaimedTask(askingTask)).toMatchObject({ id: askingTask.id, status: "running" });

      await page.addInitScript((token) => {
        localStorage.setItem("multica_token", token!);
        localStorage.setItem("multica:chat:isOpen", "false");
      }, api.getToken());
      await page.setViewportSize({ width: 1440, height: 1000 });

      const ask = async (input: Record<string, unknown> = {}) => {
        const { questions: [question] } = await api.askQuestion(askingTask.auth_token, {
          questions: [{
            prompt: "Should invoice amounts round per line or per total?",
            answer_type: "single_choice",
            settings: { options: ["per line", "per total"], allow_other: false },
            suggested_answer: { option: "per line" },
            ...input,
          }],
          answerer_ids: [userId],
          blocks_work: true,
        });
        // Arrange an ended asking run. Keep its fixture credential for the
        // agent-side read assertion; daemon completion/token revocation is not
        // under test, and no agent executable is resolved or invoked.
        await db.query(`UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1`, [askingTask.id]);
        return question;
      };
      const taskIds = async () => {
        const rows = await db.query<{ id: string }>(
          `SELECT t.id FROM agent_task_queue t JOIN agent a ON a.id = t.agent_id WHERE a.workspace_id = $1 ORDER BY t.id`,
          [workspace.id],
        );
        return rows.rows.map((row) => row.id);
      };
      await use({ api, db, workspace, userId, agentId, runtimeId, issue, askingTask, ask, taskIds });
    } finally {
      try {
        if (agentId) {
          // Question tables deliberately have no foreign-key cascades.
          await db.query(`DELETE FROM todo WHERE question_id IN (SELECT id FROM question WHERE asker_agent_id = $1)`, [agentId]);
          await db.query(`DELETE FROM question_event WHERE question_id IN (SELECT id FROM question WHERE asker_agent_id = $1)`, [agentId]);
          await db.query(`DELETE FROM question_answer WHERE question_id IN (SELECT id FROM question WHERE asker_agent_id = $1)`, [agentId]);
          await db.query(`DELETE FROM question_answerer WHERE question_id IN (SELECT id FROM question WHERE asker_agent_id = $1)`, [agentId]);
          await db.query(`DELETE FROM question_link WHERE question_id IN (SELECT id FROM question WHERE asker_agent_id = $1)`, [agentId]);
          await db.query(`DELETE FROM question WHERE asker_agent_id = $1`, [agentId]);
          await db.query(`DELETE FROM task_token WHERE agent_id = $1`, [agentId]);
          await db.query(`DELETE FROM agent_task_queue WHERE agent_id = $1`, [agentId]);
        }
        await api.cleanup();
        if (agentId) await db.query(`DELETE FROM agent WHERE id = $1`, [agentId]);
        if (runtimeId) await db.query(`DELETE FROM agent_runtime WHERE id = $1`, [runtimeId]);
      } finally {
        await db.end();
      }
    }
  },
});

test.setTimeout(120_000);

function questionCard(page: Page, prompt: string): Locator {
  return page.locator("[data-question-state]").filter({ has: page.getByText(prompt, { exact: true }) });
}

async function acceptSuggestion(page: Page, card: Locator, id: string, wake: boolean) {
  const sent = page.waitForResponse((response) =>
    response.request().method() === "POST" && response.url().endsWith(`/api/questions/${id}/answer`),
  );
  await card.getByRole("button", { name: "Accept suggestion", exact: true }).click();
  const response = await sent;
  expect(response.status()).toBe(200);
  expect(response.request().postDataJSON()).toEqual({ value: { option: "per line" }, ...(wake ? { wake: true } : {}) });
  const stored: TestQuestion = await response.json();
  expect(stored.status).toBe("answered");
  expect(stored.answers).toHaveLength(1);
  expect(stored.answers[0]).toMatchObject({ value: { option: "per line" }, display_text: "per line" });
}

async function expectFolded(card: Locator, prompt: string) {
  await expect(card).toHaveAttribute("data-question-state", "answered");
  await expect(card.getByText(prompt, { exact: true })).toBeVisible();
  await expect(card.getByText("per line", { exact: true })).toBeVisible();
  await expect(card).toContainText(/Brian · .+/);
  await expect(card.getByRole("button", { name: "Edit answer", exact: true })).toBeVisible();
  await expect(card.getByRole("radio")).toHaveCount(0);
  await expect(card.getByRole("button", { name: "Send", exact: true })).toHaveCount(0);
  await expect(card.getByRole("button", { name: "Accept suggestion", exact: true })).toHaveCount(0);
}

function openTodoCount(fixture: QuestionFixture, question: TestQuestion) {
  return fixture.db.query<{ count: number }>(
    `SELECT count(*)::int AS count FROM todo WHERE workspace_id = $1 AND member_id = $2 AND kind = 'question' AND question_id = $3 AND done_at IS NULL`,
    [fixture.workspace.id, fixture.userId, question.id],
  ).then((rows) => rows.rows[0]!.count);
}

function questionTodoRow(page: Page, fixture: QuestionFixture): Locator {
  return page.getByRole("button").filter({ hasText: fixture.issue.title }).filter({ hasText: "Question for you" });
}

async function openInboxQuestion(page: Page, fixture: QuestionFixture, question: TestQuestion) {
  // Observe the real to-do the question opened, rather than inserting or mocking one.
  await expect.poll(() => openTodoCount(fixture, question), { timeout: 30_000 }).toBe(1);
  await page.goto(`/${fixture.workspace.slug}/inbox`, { waitUntil: "domcontentloaded" });
  await questionTodoRow(page, fixture).click({ timeout: 45_000 });
  const card = questionCard(page, question.prompt);
  await expect(card).toBeVisible();
  return card;
}

test("Brian accepts Pieter's suggestion, sees the folded answer, and explicitly wakes Pieter", async ({ page, question: fixture }) => {
  const question = await fixture.ask();
  const beforeAnswer = await fixture.taskIds();
  expect(beforeAnswer).toContain(fixture.askingTask.id);
  await page.goto(`/${fixture.workspace.slug}/issues/${fixture.issue.id}`, { waitUntil: "domcontentloaded" });
  const card = questionCard(page, question.prompt);
  const run = page.locator(`[data-run-id="${fixture.askingTask.id}"]`);

  await test.step("Questions property / One blocking question and Jump to the card; Run row / Run #3 waits", async () => {
    const questionsRow = page.getByRole("button", { name: "1 open · blocks work", exact: true });
    await expect(questionsRow).toBeVisible();
    await questionsRow.click();
    await expect(card).toBeInViewport();
    await expect(card).toHaveAttribute("data-question-state", "open");
    await expect(card.getByText("Question for you", { exact: true })).toBeVisible();
    await expect(card).toContainText("asked during run #3");
    await expect(run).toContainText("ended run #3 with a question");
    await expect(run.getByText("Waiting for answer", { exact: true })).toBeVisible();
  });

  await test.step("Accept the suggested answer / Brian accepts the suggestion; Sending the last open question offers Wake / Brian unchecks Wake", async () => {
    await expect(card.getByRole("button", { name: "Wake Pieter", exact: true })).toHaveCount(0);
    const wake = card.getByRole("checkbox", { name: "Wake Pieter", exact: true });
    await expect(wake).toBeChecked();
    await wake.click();
    await expect(wake).not.toBeChecked();
    await acceptSuggestion(page, card, question.id, false);
  });

  await test.step("A sent answer reaches the asker at once / The card folds after sending", async () => {
    await expectFolded(card, question.prompt);
    const readable = await fixture.api.getQuestion(question.id, fixture.askingTask.auth_token);
    expect(readable.answers).toEqual([expect.objectContaining({
      answerer_user_id: fixture.userId,
      value: { option: "per line" },
      display_text: "per line",
    })]);
    expect(Number.isFinite(Date.parse(readable.answers[0]!.updated_at))).toBe(true);
  });

  await test.step("An answer starts a run only with Wake checked / Brian unchecks Wake; Run row / The badge leaves once answered", async () => {
    expect(await fixture.taskIds()).toEqual(beforeAnswer);
    await expect(run.getByText("Waiting for answer", { exact: true })).toHaveCount(0);
    await page.reload({ waitUntil: "domcontentloaded" });
    await expectFolded(card, question.prompt);
    expect(await fixture.taskIds()).toEqual(beforeAnswer);
  });

  await test.step("Wake the asking agent from the answered card / Wake offered after the answer", async () => {
    await expect(card.getByRole("button", { name: "Wake Pieter", exact: true })).toBeVisible();
    expect(await fixture.api.getQuestion(question.id)).toMatchObject({ wake_available: true, unseen_answer_count: 1 });
  });

  await test.step("A woken run gets the answers it has not seen / Brian wakes Pieter", async () => {
    const woke = page.waitForResponse((response) =>
      response.request().method() === "POST" && response.url().endsWith(`/api/questions/${question.id}/wake`),
    );
    await card.getByRole("button", { name: "Wake Pieter", exact: true }).click();
    const response = await woke;
    expect(response.status()).toBe(202);
    const wake: { task_id: string; answer_count: number } = await response.json();
    expect(wake.answer_count).toBe(1);
    expect(await fixture.taskIds()).toEqual([...beforeAnswer, wake.task_id].sort());
    await expect(page.getByText("Pieter was woken by Brian with 1 answer", { exact: true })).toBeVisible();
    const claimed = await fixture.api.claimRuntimeTask(fixture.runtimeId);
    expect(claimed).not.toBeNull();
    if (!claimed) throw new Error("Wake queued no claimable task");
    expect(claimed).toMatchObject({ id: wake.task_id, agent_id: fixture.agentId, issue_id: fixture.issue.id });
    expect(claimed.question_answers).toMatchObject({
      waker_name: "Brian",
      answer_count: 1,
      answers: [{ question_id: question.id, prompt: question.prompt, display_text: "per line", answerer_name: "Brian" }],
    });
    expect(Number.isFinite(Date.parse(claimed.question_answers!.answers[0]!.updated_at))).toBe(true);
    // Acknowledge the claim through the API, not an installed agent executable.
    expect(await fixture.api.startClaimedTask(claimed)).toMatchObject({ id: wake.task_id, status: "running" });
    await page.reload({ waitUntil: "domcontentloaded" });
    await expectFolded(card, question.prompt);
    await expect(card.getByRole("button", { name: "Wake Pieter", exact: true })).toHaveCount(0);
  });
});

test("Answer simple questions in the inbox / Brian answers from the inbox", async ({ page, question: fixture }) => {
  const question = await fixture.ask();
  const beforeAnswer = await fixture.taskIds();
  const card = await openInboxQuestion(page, fixture, question);
  await expect(card.getByRole("radio", { name: /per line/ })).toBeVisible();
  await expect(card.getByRole("checkbox", { name: "Wake Pieter", exact: true })).toBeChecked();
  await acceptSuggestion(page, card, question.id, true);
  // Answering handles the to-do: it leaves the Inbox.
  await expect.poll(() => openTodoCount(fixture, question), { timeout: 30_000 }).toBe(0);
  await expect(questionTodoRow(page, fixture)).toHaveCount(0, { timeout: 30_000 });
  await expect(page).toHaveURL(new RegExp(`/${fixture.workspace.slug}/inbox`));
  // Sent with Wake Pieter checked, the answer queues one run of Pieter.
  const afterAnswer = await fixture.taskIds();
  expect(afterAnswer).toEqual(expect.arrayContaining(beforeAnswer));
  expect(afterAnswer).toHaveLength(beforeAnswer.length + 1);
  expect((await fixture.api.getQuestion(question.id)).answers[0]).toMatchObject({ answerer_user_id: fixture.userId, value: { option: "per line" } });

  await page.goto(`/${fixture.workspace.slug}/issues/${fixture.issue.id}`, { waitUntil: "domcontentloaded" });
  await expectFolded(questionCard(page, question.prompt), question.prompt);
  expect(await fixture.taskIds()).toEqual(afterAnswer);
});

test("Answer simple questions in the inbox / Other types open the issue", async ({ page, question: fixture }) => {
  const question = await fixture.ask({
    prompt: "Which invoice work should Pieter do first?",
    answer_type: "ranking",
    settings: { items: ["Round amounts", "Show totals", "Export invoices"] },
    suggested_answer: null,
  });
  const beforeOpen = await fixture.taskIds();
  const card = await openInboxQuestion(page, fixture, question);
  await expect(card.getByText("Ranking is answered on its issue.", { exact: true })).toBeVisible();
  await expect(card.getByRole("button", { name: "Send", exact: true })).toHaveCount(0);
  await expect(card.getByRole("button", { name: "Accept suggestion", exact: true })).toHaveCount(0);
  await expect(card.getByRole("list", { name: "Order", exact: true })).toHaveCount(0);
  await card.getByRole("button", { name: `Open on ${fixture.issue.identifier}`, exact: true }).click();
  // The inbox mounts the real issue detail in its split pane rather than
  // changing the route. The question's timeline anchor proves the landing.
  const landed = page.locator(`#comment-question-${question.id}`);
  await expect(landed).toBeInViewport();
  await expect(landed.getByText(question.prompt, { exact: true })).toBeVisible();
  await expect(landed.getByRole("button", { name: "Send", exact: true })).toBeVisible();
  await expect(landed.getByRole("list", { name: "Order", exact: true })).toBeVisible();
  expect(await fixture.api.getQuestion(question.id)).toMatchObject({ status: "open", answers: [] });
  expect(await fixture.taskIds()).toEqual(beforeOpen);
});
