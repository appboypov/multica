/** @vitest-environment jsdom */
import { act, cleanup, renderHook, type RenderHookResult } from "@testing-library/react";
import { QueryClientProvider, useQuery } from "@tanstack/react-query";
import type { QueryClient } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import { createQueryClient } from "../query-client";
import type { Todo, TodoWorkspaceCount } from "../types/todo";
import { useMarkTodoDone } from "./mutations";
import {
  hasOtherWorkspaceTodos,
  todoKeys,
  todoListOptions,
  todoSummaryOptions,
  todoWorkspaceIds,
  useTodoCount,
} from "./queries";
import { onTodoClosed, onTodoOpened } from "./ws-updaters";

/**
 * The Inbox number, the workspace switcher dot and the desktop dock badge all
 * read `GET /api/todos/summary`; the Inbox page reads `GET /api/todos`. Two
 * independent server reads, rendered side by side. This pins the property
 * that makes that safe: once everything settles, the number equals the rows
 * the list shows, the switcher dot equals "another workspace has an open
 * to-do", and all of them equal the server — whatever order responses and
 * `todo:*` events arrive in.
 *
 * The list's first load is exercised repeatedly on purpose: nothing outside
 * the Inbox page observes the list, so on web the cache is collected while the
 * member is elsewhere and every return is a first load again. A to-do closing
 * by itself (`done_by = handled`) and one closing in another workspace arrive
 * only as `todo:closed`, which carries no workspace id.
 */

vi.mock("../hooks", () => ({ useWorkspaceId: () => "ws-1" }));
afterEach(cleanup);

const WS = "ws-1";
const OTHER_WS = "ws-2";
// Deterministic, so the range is a fixed set of interleavings that replays
// exactly by seed.
const SEEDS = 25;

function mulberry32(seed: number) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/**
 * A server with the rules of ListOpenTodos and CountOpenTodosByWorkspace. Each
 * response is computed when the request ARRIVES but handed back only when the
 * scheduler releases it — the adversarial case, where a response describing
 * an older state lands after newer writes.
 */
class FakeServer {
  rows: Todo[] = [];
  private clock = 0;
  private seq = 0;

  open(workspaceId: string): Todo {
    // Strictly increasing: each INSERT gets its own now().
    this.clock += 1;
    const row: Todo = {
      id: `t${++this.seq}`,
      workspace_id: workspaceId,
      kind: "mention",
      issue_id: `issue-${this.seq}`,
      issue_identifier: `WORK-${this.seq}`,
      issue_title: `Issue ${this.seq}`,
      issue_status: "todo",
      question_id: null,
      question_answer_type: null,
      comment_id: `comment-${this.seq}`,
      task_id: null,
      autopilot_id: null,
      actor_type: "agent",
      actor_id: "agent",
      title: `Issue ${this.seq}`,
      body: null,
      details: {},
      due_date: null,
      overdue: false,
      created_at: new Date(Date.UTC(2026, 9, 1, 0, 0, this.clock)).toISOString(),
      done_at: null,
      done_by: null,
    };
    this.rows.push(row);
    return row;
  }

  close(id: string, doneBy: "handled" | "member"): Todo | undefined {
    const row = this.rows.find((r) => r.id === id);
    if (row && !row.done_at) {
      row.done_at = new Date(Date.UTC(2026, 9, 2)).toISOString();
      row.done_by = doneBy;
    }
    return row;
  }

  openRows(): Todo[] {
    return this.rows.filter((r) => !r.done_at);
  }

  list(workspaceId: string): Todo[] {
    return this.openRows()
      .filter((r) => r.workspace_id === workspaceId)
      .map((r) => ({ ...r }))
      .sort((a, b) => b.created_at.localeCompare(a.created_at));
  }

  count(workspaceId: string): number {
    return this.list(workspaceId).length;
  }

  summary(): TodoWorkspaceCount[] {
    return [WS, OTHER_WS]
      .map((workspace_id) => ({ workspace_id, count: this.count(workspace_id) }))
      .filter((entry) => entry.count > 0);
  }
}

async function tick() {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
}

it.each(Array.from({ length: SEEDS }, (_, i) => i + 1))(
  "Inbox number, switcher dot and listed rows converge on the server (seed %i)",
  async (seed) => {
    const rand = mulberry32(seed);
    const server = new FakeServer();
    for (const ws of [WS, WS, WS, OTHER_WS]) server.open(ws);
    server.close("t2", "handled");

    const pending: Array<() => void> = [];
    const events: Array<(qc: QueryClient) => void> = [];
    const qc = createQueryClient();

    const deferred = <T,>(value: () => T): Promise<T> => {
      const snapshot = value();
      const { promise, resolve } = Promise.withResolvers<T>();
      pending.push(() => resolve(snapshot));
      return promise;
    };

    setApiInstance({
      listTodos: vi.fn(() => deferred(() => server.list(WS))),
      getTodoSummary: vi.fn(() => deferred(() => server.summary())),
      markTodoDone: vi.fn((id: string) => {
        // Commit, then publish: the event exists before the response does.
        server.close(id, "member");
        events.push((c) => void onTodoClosed(c, id));
        return deferred(() => ({ ...server.rows.find((r) => r.id === id)! }));
      }),
    } as unknown as ApiClient);

    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    // The sidebar and the desktop badge: always mounted, read only the summary.
    const shell = renderHook(
      () => {
        const summary = useQuery(todoSummaryOptions()).data ?? [];
        return {
          number: useTodoCount(WS),
          switcherDot: hasOtherWorkspaceTodos(summary, WS),
          dottedWorkspaces: todoWorkspaceIds(summary),
          markDone: useMarkTodoDone(),
        };
      },
      { wrapper },
    );
    // The Inbox page: the list's only observer, so it comes and goes.
    const mountPage = () =>
      renderHook(() => useQuery(todoListOptions(WS)).data, { wrapper });
    let page: RenderHookResult<Todo[] | undefined, unknown> | null = mountPage();

    try {
      for (let step = 0; step < 40; step++) {
        const roll = rand();
        if (roll < 0.16) {
          // A to-do opens, in this workspace or another one.
          const row = server.open(rand() < 0.6 ? WS : OTHER_WS);
          events.push((c) => void onTodoOpened(c, row));
        } else if (roll < 0.26) {
          // A to-do closes by itself (answered, handled elsewhere), in any
          // workspace; its event carries no workspace id.
          const open = server.openRows();
          if (open.length > 0) {
            const target = open[Math.floor(rand() * open.length)]!;
            server.close(target.id, "handled");
            events.push((c) => void onTodoClosed(c, target.id));
          }
        } else if (roll < 0.38 && page) {
          // The member marks a listed row done.
          const listed = page.result.current ?? [];
          if (listed.length > 0) {
            const target = listed[Math.floor(rand() * listed.length)]!;
            act(() => shell.result.current.markDone.mutate(target.id));
          }
        } else if (roll < 0.46) {
          // Leave the Inbox long enough for the list cache to be collected,
          // then come back — the next read is a first load again.
          if (page) {
            page.unmount();
            page = null;
            qc.removeQueries({ queryKey: todoKeys.list(WS), exact: true });
          } else {
            page = mountPage();
          }
        } else if (roll < 0.78 && pending.length > 0) {
          // Release a random in-flight response — possibly a stale one.
          const [release] = pending.splice(Math.floor(rand() * pending.length), 1);
          await act(async () => release!());
        } else if (events.length > 0) {
          // Deliver the next event, in publish order.
          const event = events.shift()!;
          await act(async () => event(qc));
        }
        await tick();
      }

      // Drain: every event and response is eventually delivered, and the
      // member ends up back on the Inbox page.
      if (!page) page = mountPage();
      for (let guard = 0; guard < 500 && (pending.length || events.length); guard++) {
        const event = events.shift();
        if (event) await act(async () => event(qc));
        while (pending.length) {
          const [release] = pending.splice(Math.floor(rand() * pending.length), 1);
          await act(async () => release!());
        }
        for (let i = 0; i < 4; i++) await tick();
      }
      for (let i = 0; i < 6; i++) await tick();

      const truth = server.list(WS).map((r) => r.id);
      const otherOpen = server.count(OTHER_WS) > 0;
      expect({
        number: shell.result.current.number,
        listed: (page.result.current ?? []).map((r) => r.id),
        switcherDot: shell.result.current.switcherDot,
        otherWorkspaceDotted: shell.result.current.dottedWorkspaces.has(OTHER_WS),
      }).toEqual({
        number: truth.length,
        listed: truth,
        switcherDot: otherOpen,
        otherWorkspaceDotted: otherOpen,
      });
    } finally {
      for (const release of pending) release();
      page?.unmount();
      shell.unmount();
      qc.clear();
    }
  },
);
