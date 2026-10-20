// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  AppConfigSchema,
  AgentTaskListSchema,
  AgentActivityBucketListSchema,
  TaskMessageListSchema,
  AutopilotQuotaUsageSchema,
  AutopilotRunSchema,
  FALLBACK_AUTOPILOT_RUN,
  CommentTriggerPreviewSchema,
  CommentSchema,
  DashboardAgentRunTimeListSchema,
  DashboardRunTimeDailyListSchema,
  DashboardFailureByAgentListSchema,
  DashboardFailureDailyListSchema,
  DashboardUsageByAgentListSchema,
  DashboardUsageDailyListSchema,
  ChatDraftRestoresResponseSchema,
  ChatPendingTaskSchema,
  ChatSessionListSchema,
  ChatSessionSchema,
  PrioritizeQueuedChatTaskResponseSchema,
  CreateFeedbackResponseSchema,
  DuplicateIssueErrorBodySchema,
  EMPTY_CHAT_DRAFT_RESTORES,
  EMPTY_CHAT_PENDING_TASK,
  EMPTY_CHAT_SESSION,
  EMPTY_PRIORITIZE_QUEUED_CHAT_TASK_RESPONSE,
  EMPTY_CREATE_FEEDBACK_RESPONSE,
  EMPTY_INBOX_ITEMS,
  EMPTY_INBOX_UNREAD_SUMMARY,
  EMPTY_LIST_ORGANISATIONS_RESPONSE,
  EMPTY_ORGANISATION,
  EMPTY_SEARCH_PROJECTS_RESPONSE,
  EMPTY_USER,
  InboxItemListSchema,
  InboxUnreadSummarySchema,
  IssueTriggerPreviewSchema,
  ListIssuesResponseSchema,
  ListPropertiesResponseSchema,
  ListOrganisationsResponseSchema,
  OrganisationSchema,
  MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
  RuntimeModelListRequestSchema,
  RuntimeDirectoryListingRequestSchema,
  MALFORMED_RUNTIME_DIRECTORY_LISTING_REQUEST,
  SearchProjectsResponseSchema,
  RuntimeHourlyActivityListSchema,
  RuntimeUsageByAgentListSchema,
  RuntimeUsageByHourListSchema,
  RuntimeUsageListSchema,
  SendChatMessageResponseSchema,
  SquadListSchema,
  SquadMemberListSchema,
  SquadSchema,
  SourceContextPreviewSchema,
  TimelineEntriesSchema,
  UserSchema,
  PluginInstallationSchema,
  PluginInstallationListResponseSchema,
  PluginMCPToolListSchema,
  PluginPreviewSchema,
  EMPTY_PLUGIN_INSTALLATION_LIST,
  EMPTY_PLUGIN_PREVIEW,
} from "./schemas";
import { IssueViewSchema, IssueViewListSchema } from "./schemas";
import {
  ListIssueStatusesResponseSchema,
  IssueStatusEntrySchema,
  EMPTY_LIST_ISSUE_STATUSES_RESPONSE,
  EMPTY_ISSUE_STATUS_ENTRY,
} from "./schemas";
import {
  EMPTY_QUESTION,
  EMPTY_QUESTION_LIST,
  EMPTY_QUESTION_ROW_PAGE,
  EMPTY_QUESTION_WAKE_RESULT,
  QuestionListSchema,
  QuestionRowPageSchema,
  QuestionSchema,
  QuestionWakeResultSchema,
  RepositoryCheckoutRequestSchema,
  MALFORMED_REPOSITORY_CHECKOUT_REQUEST,
} from "./schemas";
import {
  AgentSessionsResponseSchema,
  EMPTY_AGENT_SESSIONS_RESPONSE,
} from "./schemas";
import type {
  AgentSessionsResponse,
  Question,
  QuestionRowPage,
  QuestionWakeResult,
  RepositoryCheckoutRequest,
} from "../types";
import { parseWithFallback } from "./schema";
import {
  EMPTY_RELEASE,
  EMPTY_RELEASE_ACTIVITY_RESPONSE,
  EMPTY_RELEASE_DETAIL_RESPONSE,
  ReleaseActivityResponseSchema,
  ReleaseCommentsSchema,
  ReleaseDetailResponseSchema,
  ReleaseIssuesResponseSchema,
  ReleasesResponseSchema,
} from "./schemas";

const baseIssue = {
  id: "11111111-1111-1111-1111-111111111111",
  workspace_id: "ws-1",
  number: 1,
  identifier: "MUL-1",
  title: "Test",
  description: null,
  status: "todo",
  priority: "medium",
  assignee_type: null,
  assignee_id: null,
  creator_type: "member",
  creator_id: "user-1",
  parent_issue_id: null,
  project_id: null,
  position: 0,
  stage: null,
  start_date: null,
  due_date: null,
  metadata: {},
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

describe("ChatSessionSchema", () => {
  const baseSession = {
    id: "chat-1",
    workspace_id: "ws-1",
    agent_id: "agent-1",
    creator_id: "user-1",
    title: "Chat",
    status: "active",
    has_unread: false,
    created_at: "2026-08-18T00:00:00Z",
    updated_at: "2026-08-18T00:00:00Z",
  };

  it("degrades a malformed session to the empty Chat", () => {
    expect(parseWithFallback({ id: 42 }, ChatSessionSchema, EMPTY_CHAT_SESSION, {
      endpoint: "GET /api/chat/sessions/:id",
    })).toEqual(EMPTY_CHAT_SESSION);
  });

  it("drops one malformed list item without hiding the other Chats", () => {
    const parsed = ChatSessionListSchema.parse([
      baseSession,
      { ...baseSession, id: 42 },
      {
        ...baseSession,
        id: "onboarding-chat",
        last_message: {
          content: "Welcome",
          role: "assistant",
          created_at: "2026-08-18T00:00:00Z",
          message_kind: "onboarding_opening",
        },
      },
    ]);

    expect(parsed.map((session) => session.id)).toEqual(["chat-1", "onboarding-chat"]);
    expect(parsed[1]?.last_message?.message_kind).toBe("onboarding_opening");
  });
});
describe("IssueSchema (via ListIssuesResponseSchema)", () => {
  // A custom status key can be derived rather than readable — "客户确认" becomes
  // `in_review_2` — so the display name travels with it. The field has to
  // survive a server that predates it, since an issue that fails validation
  // degrades to a stub rather than losing one field. (MUL-6749)
  it("carries a custom status's display name", () => {
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{ ...baseIssue, status: "in_review_2", status_name: "客户确认" }],
      total: 1,
    });
    expect(parsed.issues[0]?.status_name).toBe("客户确认");
  });
  it("drops only a malformed status_name, keeping the issue and the list", () => {
    for (const bad of [42, { name: "x" }, ["x"], true]) {
      const parsed = ListIssuesResponseSchema.parse({
        issues: [{ ...baseIssue, status: "in_review_2", status_name: bad }],
        total: 1,
      });
      expect(parsed.issues).toHaveLength(1);
      expect(parsed.issues[0]?.id).toBe(baseIssue.id);
      expect(parsed.issues[0]?.status).toBe("in_review_2");
      expect(parsed.issues[0]?.status_name).toBeUndefined();
    }
  });
  it("still parses an issue from a server that does not send status_name", () => {
    const { status_name: _omitted, ...withoutName } = { ...baseIssue, status_name: "x" };
    const parsed = ListIssuesResponseSchema.parse({ issues: [withoutName], total: 1 });
    expect(parsed.issues[0]?.id).toBe(baseIssue.id);
    expect(parsed.issues[0]?.status_name).toBeUndefined();
  });
  it("keeps the issue while independently dropping a malformed source context", () => {
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{ ...baseIssue, source_context: { snapshot: "bad" } }],
      total: 1,
    });
    expect(parsed.issues[0]?.id).toBe(baseIssue.id);
    expect(parsed.issues[0]?.source_context).toBeUndefined();
  });
  it("parses source-context change reasons without requiring them from older servers", () => {
    const sourceContext = {
      id: "context-1",
      version: 1,
      usage: "read_only_historical_background",
      captured_at: "2026-08-21T12:00:00Z",
      display_state: "changed",
      source_issue_state: "changed",
      comment_thread_state: "unchanged",
      anchor_comment_state: "available",
      can_open_current_source: true,
      change_reasons: ["issue_description_attachments"],
      change_details: {
        changed_comment_ids: ["comment-1"],
        added_comments: [{
          id: "comment-2", parent_id: "comment-1", type: "comment", content: "new reply",
          author: { type: "member", id: "user-2", name: "Bob" },
          created_at: "later", updated_at: "later", revision: 1, attachments: [],
        }],
        removed_comment_ids: ["comment-3"],
        description_attachment_changes: [{
          kind: "removed", attachment_id: "attachment-1", filename: "old.txt",
        }],
      },
      snapshot: {
        source_issue: {
          id: "issue-1", identifier: "MUL-1", number: 1, title: "Source",
          description: null, created_at: "now", updated_at: "now", revision: 1,
          attachments: [],
        },
        comment_thread: [{
          id: "comment-1", parent_id: null, type: "comment", content: "history",
          author: { type: "member", id: "user-1", name: "Alice" },
          created_at: "now", updated_at: "now", revision: 1, attachments: [],
        }],
        anchor_comment_id: "comment-1",
      },
    };
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{ ...baseIssue, source_context: sourceContext }],
      total: 1,
    });
    expect(parsed.issues[0]?.source_context?.change_reasons).toEqual(["issue_description_attachments"]);
    expect(parsed.issues[0]?.source_context?.change_details).toEqual(sourceContext.change_details);

    const { change_reasons: _reasonsOmitted, change_details: _detailsOmitted, ...legacyContext } = sourceContext;
    const legacy = ListIssuesResponseSchema.parse({
      issues: [{ ...baseIssue, source_context: legacyContext }],
      total: 1,
    });
    expect(legacy.issues[0]?.source_context?.change_reasons).toBeUndefined();
    expect(legacy.issues[0]?.source_context?.change_details).toBeUndefined();

    const malformed = ListIssuesResponseSchema.parse({
      issues: [{
        ...baseIssue,
        source_context: {
          ...sourceContext,
          change_details: { ...sourceContext.change_details, added_comments: [{ id: 42 }] },
        },
      }],
      total: 1,
    });
    expect(malformed.issues[0]?.source_context).toBeUndefined();
  });
  it("accepts null activity during backfill and rejects malformed activity", () => {
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{ ...baseIssue, last_activity_at: null }],
      total: 1,
    });
    expect(parsed.issues[0]?.last_activity_at).toBeNull();
    expect(() =>
      ListIssuesResponseSchema.parse({
        issues: [{ ...baseIssue, last_activity_at: 42 }],
        total: 1,
      }),
    ).toThrow();
  });

  it("accepts a primitive metadata KV map", () => {
    const payload = {
      issues: [
        {
          ...baseIssue,
          metadata: { pipeline_status: "waiting", pr_number: 3, is_blocked: true },
        },
      ],
      total: 1,
    };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.metadata).toEqual({
      pipeline_status: "waiting",
      pr_number: 3,
      is_blocked: true,
    });
  });

  it("defaults metadata to {} when the server omits it (older backend)", () => {
    const { metadata: _omit, ...issueWithoutMetadata } = baseIssue;
    const payload = { issues: [issueWithoutMetadata], total: 1 };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.metadata).toEqual({});
  });

  it("rejects metadata with non-primitive values (nested object)", () => {
    const payload = {
      issues: [{ ...baseIssue, metadata: { nested: { x: 1 } } }],
      total: 1,
    };
    expect(ListIssuesResponseSchema.safeParse(payload).success).toBe(false);
  });

  it("accepts a numeric stage", () => {
    const payload = { issues: [{ ...baseIssue, stage: 2 }], total: 1 };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.stage).toBe(2);
  });

  it("defaults stage to null when the server omits it (older backend)", () => {
    const { stage: _omit, ...issueWithoutStage } = baseIssue;
    const payload = { issues: [issueWithoutStage], total: 1 };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.stage).toBeNull();
  });

  it("accepts custom property values including multi_select arrays", () => {
    const payload = {
      issues: [
        {
          ...baseIssue,
          properties: { "def-1": "opt-a", "def-2": ["opt-x", "opt-y"], "def-3": 3.5, "def-4": true },
        },
      ],
      total: 1,
    };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.properties).toEqual({
      "def-1": "opt-a",
      "def-2": ["opt-x", "opt-y"],
      "def-3": 3.5,
      "def-4": true,
    });
  });

  it("defaults properties to {} when the server omits it (older backend)", () => {
    const parsed = ListIssuesResponseSchema.parse({ issues: [baseIssue], total: 1 });
    expect(parsed.issues[0]?.properties).toEqual({});
  });

  it("drops unknown-shaped property values instead of failing the issue parse", () => {
    // Forward compat: a future server type (actor/relation) may ship object
    // values. That one entry must disappear; the issue and its other
    // properties must survive — a full parse failure would blank the whole
    // list through parseWithFallback on installed desktop builds.
    const payload = {
      issues: [
        {
          ...baseIssue,
          properties: { "def-1": { nested: 1 }, "def-2": "opt-a" },
        },
      ],
      total: 1,
    };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.properties).toEqual({ "def-2": "opt-a" });
  });
});

describe("SourceContextPreviewSchema", () => {
  it("parses a thread-history preview and rejects a missing token", () => {
    const preview = {
      source_issue: {
        id: "issue-1", identifier: "MUL-1", number: 1, title: "Source",
        description: null, created_at: "now", updated_at: "now", revision: 1,
        attachments: [],
      },
      comment_thread: [{
        id: "comment-1", parent_id: null, type: "comment", content: "history",
        author: { type: "member", id: "user-1", name: "Alice" },
        created_at: "now", updated_at: "now", revision: 1, attachments: [],
      }],
      anchor_comment_id: "comment-1",
      capture_token: "sha256:token",
      limits: { comment_count: 1, text_bytes: 7, attachment_count: 0, attachment_bytes: 0 },
    };
    expect(SourceContextPreviewSchema.parse(preview).comment_thread).toHaveLength(1);
    expect(() => SourceContextPreviewSchema.parse({ ...preview, capture_token: "" })).toThrow();
  });

  it("normalizes null attachment lists from early source-context servers", () => {
    const preview = {
      source_issue: {
        id: "issue-1", identifier: "MUL-1", number: 1, title: "Source",
        description: null, created_at: "now", updated_at: "now", revision: 1,
        attachments: null,
      },
      comment_thread: [{
        id: "comment-1", parent_id: null, type: "comment", content: "history",
        author: { type: "member", id: "user-1", name: "Alice" },
        created_at: "now", updated_at: "now", revision: 1, attachments: null,
      }],
      anchor_comment_id: "comment-1",
      capture_token: "sha256:token",
      limits: { comment_count: 1, text_bytes: 7, attachment_count: 0, attachment_bytes: 0 },
    };

    const parsed = SourceContextPreviewSchema.parse(preview);
    expect(parsed.source_issue.attachments).toEqual([]);
    expect(parsed.comment_thread[0]?.attachments).toEqual([]);
  });
});

describe("IssuePropertySchema (via ListPropertiesResponseSchema)", () => {
  const baseProperty = {
    id: "22222222-2222-2222-2222-222222222222",
    workspace_id: "ws-1",
    name: "Severity",
    type: "select",
    description: "",
    icon: "flag",
    config: { options: [{ id: "opt-1", name: "Critical", color: "#ef4444" }] },
    position: 1,
    archived: false,
    archived_at: null,
    usage_count: 2,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };

  it("parses a full definition", () => {
    const parsed = ListPropertiesResponseSchema.parse({ properties: [baseProperty], total: 1 });
    expect(parsed.properties[0]?.config.options?.[0]?.name).toBe("Critical");
    expect(parsed.properties[0]?.icon).toBe("flag");
  });

  it("survives a malformed response by defaulting the list", () => {
    const parsed = ListPropertiesResponseSchema.parse({});
    expect(parsed.properties).toEqual([]);
    expect(parsed.total).toBe(0);
  });

  it("keeps unknown property types as strings (forward compat)", () => {
    const parsed = ListPropertiesResponseSchema.parse({
      properties: [{ ...baseProperty, type: "relation", config: {} }],
      total: 1,
    });
    expect(parsed.properties[0]?.type).toBe("relation");
  });

  it("defaults config when the server sends none", () => {
    const { config: _omit, ...withoutConfig } = baseProperty;
    const parsed = ListPropertiesResponseSchema.parse({ properties: [withoutConfig], total: 1 });
    expect(parsed.properties[0]?.config).toEqual({});
  });

  it("defaults icon for an older server response", () => {
    const { icon: _omit, ...withoutIcon } = baseProperty;
    const parsed = ListPropertiesResponseSchema.parse({ properties: [withoutIcon], total: 1 });
    expect(parsed.properties[0]?.icon).toBe("");
  });
});

// POST /api/issues/preview-trigger feeds this schema through parseWithFallback
// in client.previewIssueTrigger with fallback { triggers: [], total_count: 0 }
// (MUL-3375). The four entry points read it to decide "will this start a run",
// so malformed / missing / null drift must degrade to "nothing will start"
// rather than throw into the picker/modal.
const PREVIEW_FALLBACK = { triggers: [], total_count: 0 };
const PREVIEW_ENDPOINT = { endpoint: "POST /api/issues/preview-trigger" };

describe("IssueTriggerPreviewSchema", () => {
  it("parses a well-formed response", () => {
    const parsed = IssueTriggerPreviewSchema.parse({
      triggers: [
        { issue_id: "i1", agent_id: "a1", source: "assign" },
        { issue_id: "i2", agent_id: "a2", source: "status" },
      ],
      total_count: 2,
    });
    expect(parsed.total_count).toBe(2);
    expect(parsed.triggers).toHaveLength(2);
    expect(parsed.triggers[0]).toMatchObject({ issue_id: "i1", agent_id: "a1", source: "assign" });
  });

  it("defaults missing top-level fields (empty / older backend)", () => {
    const parsed = IssueTriggerPreviewSchema.parse({});
    expect(parsed.triggers).toEqual([]);
    expect(parsed.total_count).toBe(0);
  });

  it("defaults missing optional item fields, keeping required issue_id", () => {
    const parsed = IssueTriggerPreviewSchema.parse({ triggers: [{ issue_id: "i1" }], total_count: 1 });
    expect(parsed.triggers[0]).toEqual({
      issue_id: "i1",
      agent_id: "",
      source: "",
    });
  });

  it("parseWithFallback returns the fallback for a malformed shape (triggers not an array)", () => {
    const parsed = parseWithFallback(
      { triggers: "nope", total_count: 1 },
      IssueTriggerPreviewSchema,
      PREVIEW_FALLBACK,
      PREVIEW_ENDPOINT,
    );
    expect(parsed).toEqual(PREVIEW_FALLBACK);
  });

  it("parseWithFallback returns the fallback when an item drops the required issue_id", () => {
    const parsed = parseWithFallback(
      { triggers: [{ agent_id: "a1", source: "assign" }], total_count: 1 },
      IssueTriggerPreviewSchema,
      PREVIEW_FALLBACK,
      PREVIEW_ENDPOINT,
    );
    expect(parsed).toEqual(PREVIEW_FALLBACK);
  });

  it("parseWithFallback returns the fallback for a wrong-typed total_count", () => {
    const parsed = parseWithFallback(
      { triggers: [], total_count: "5" },
      IssueTriggerPreviewSchema,
      PREVIEW_FALLBACK,
      PREVIEW_ENDPOINT,
    );
    expect(parsed).toEqual(PREVIEW_FALLBACK);
  });

  it("parseWithFallback returns the fallback for null / non-object bodies", () => {
    expect(parseWithFallback(null, IssueTriggerPreviewSchema, PREVIEW_FALLBACK, PREVIEW_ENDPOINT)).toEqual(PREVIEW_FALLBACK);
    expect(parseWithFallback("oops", IssueTriggerPreviewSchema, PREVIEW_FALLBACK, PREVIEW_ENDPOINT)).toEqual(PREVIEW_FALLBACK);
  });
});

describe("run input metadata", () => {
  const timelineEntry = {
    type: "comment", id: "comment-1", actor_type: "member", actor_id: "user-1",
    created_at: "2026-10-15T10:00:00Z", content: "keep the toggle",
  };
  const comment = {
    id: "comment-1", issue_id: "issue-1", author_type: "member", author_id: "user-1",
    content: "keep the toggle", type: "comment", parent_id: null,
    created_at: timelineEntry.created_at, updated_at: timelineEntry.created_at,
  };
  const receipts = [
    { task_id: "run-1", agent_id: "thomas", item_kind: "comment", item_id: "comment-1", status: "delivered",
      seen_at: "2026-10-15T10:01:00Z", handled_at: "2026-10-15T10:02:00Z" },
    { task_id: "run-1", agent_id: "thomas", item_kind: "comment_edit", item_id: "edit-1", status: "delivered",
      seen_at: "2026-10-15T10:03:00Z" },
  ];

  it("preserves original and edited comment receipt identities and marks at both boundaries", () => {
    expect(CommentSchema.parse({ ...comment, supplements: receipts }).supplements).toEqual(receipts);
    expect(TimelineEntriesSchema.parse([{ ...timelineEntry, supplements: receipts }])[0]?.supplements).toEqual(receipts);
  });

  it("keeps comments and valid receipts when metadata is absent, unknown or malformed", () => {
    const supplements = [
      receipts[0], null, { task_id: 42, status: "delivered" },
      { task_id: "run-2", status: "delivered", item_kind: "future_kind", item_id: 3, seen_at: {}, handled_at: null },
      { task_id: "run-3", status: "delivered" },
    ];
    const parsed = CommentSchema.parse({ ...comment, supplements });
    expect(parsed.id).toBe(comment.id);
    expect(parsed.supplements).toHaveLength(3);
    expect(parsed.supplements?.[0]).toEqual(receipts[0]);
    expect(parsed.supplements?.[1]).toMatchObject({
      task_id: "run-2", item_kind: undefined, item_id: undefined, seen_at: undefined, handled_at: undefined,
    });
    expect(parsed.supplements?.[2]).toMatchObject({ task_id: "run-3", status: "delivered" });
    expect(CommentSchema.parse(comment).supplements).toBeUndefined();
  });

  it("preserves per-run activity marks and drops only malformed marks", () => {
    const marks = [
      { task_id: "run-1", agent_id: "thomas", seen_at: "2026-10-15T10:01:00Z" },
      { task_id: "run-2", agent_id: "thomas", handled_at: "2026-10-15T10:02:00Z" },
    ];
    const [parsed] = TimelineEntriesSchema.parse([{
      ...timelineEntry, type: "activity", action: "priority_changed",
      run_marks: [marks[0], null, { agent_id: "missing-run" }, marks[1],
        { task_id: "run-3", agent_id: "joeri", seen_at: 3, handled_at: null }],
    }]);
    expect(parsed?.run_marks).toEqual([
      ...marks, { task_id: "run-3", agent_id: "joeri", seen_at: undefined, handled_at: undefined },
    ]);
  });

  it.each([undefined, null, "not-a-list", 42])("ignores an unreadable activity marks list: %j", (run_marks) => {
    const [parsed] = TimelineEntriesSchema.parse([{ ...timelineEntry, type: "activity", run_marks }]);
    expect(parsed?.id).toBe(timelineEntry.id);
    expect(parsed?.run_marks).toBeUndefined();
  });

  it.each([
    ["label_added", { label_id: "label-1", label_name: "Design" }],
    ["label_removed", { label_id: "label-1", label_name: "Design" }],
    ["attachment_added", { attachment_id: "file-1", filename: "flow.pdf" }],
    ["attachment_removed", { attachment_id: "file-1", filename: "flow.pdf" }],
    ["sub_issue_added", { sub_issue_id: "child-1", sub_issue_identifier: "WORK-450", sub_issue_title: "Copy button tests" }],
    ["sub_issue_removed", { sub_issue_id: "child-1", sub_issue_identifier: "WORK-450", sub_issue_title: "Copy button tests" }],
  ])("preserves the %s activity and its snapshot", (action, details) => {
    const [parsed] = TimelineEntriesSchema.parse([{ ...timelineEntry, type: "activity", action, details }]);
    expect(parsed).toMatchObject({ action, details });
  });
});

describe("TimelineEntriesSchema", () => {
  it("preserves run-bound supplement delivery receipts", () => {
    const parsed = TimelineEntriesSchema.parse([{
      type: "comment",
      id: "supplement-1",
      actor_type: "member",
      actor_id: "user-1",
      created_at: "2026-01-01T00:00:00Z",
      content: "also cover rollback",
      supplement_task_id: "task-1",
      supplement_status: "delivered",
      supplement_delivered_at: "2026-01-01T00:00:01Z",
    }]);
    expect(parsed[0]).toMatchObject({
      supplement_task_id: "task-1",
      supplement_status: "delivered",
      supplement_delivered_at: "2026-01-01T00:00:01Z",
    });
  });

  it("preserves source_task_id for agent failure comments", () => {
    const parsed = TimelineEntriesSchema.parse([
      {
        type: "comment",
        id: "comment-1",
        actor_type: "agent",
        actor_id: "agent-1",
        created_at: "2026-01-01T00:00:00Z",
        content: "API Error: 500 Internal server error",
        comment_type: "system",
        source_task_id: "task-1",
      },
    ]);

    expect(parsed[0]?.source_task_id).toBe("task-1");
  });

  it("preserves server-hydrated historical actor identity", () => {
    const parsed = TimelineEntriesSchema.parse([
      {
        type: "comment",
        id: "comment-1",
        actor_type: "member",
        actor_id: "former-user-1",
        actor_name: "Former Member",
        actor_avatar_url: "https://profiles.example.com/former.png",
        created_at: "2026-01-01T00:00:00Z",
        content: "Authored before leaving",
      },
    ]);

    expect(parsed[0]?.actor_name).toBe("Former Member");
    expect(parsed[0]?.actor_avatar_url).toBe(
      "https://profiles.example.com/former.png",
    );
  });

  it("preserves the deleted-comment tombstone marker", () => {
    const parsed = TimelineEntriesSchema.parse([
      {
        type: "comment",
        id: "comment-1",
        actor_type: "member",
        actor_id: "user-1",
        created_at: "2026-01-01T00:00:00Z",
        content: "",
        deleted_at: "2026-01-02T00:00:00Z",
      },
    ]);

    expect(parsed[0]?.deleted_at).toBe("2026-01-02T00:00:00Z");
  });

  it("reads a malformed tombstone marker as a live comment instead of failing the timeline", () => {
    const parsed = TimelineEntriesSchema.parse([
      {
        type: "comment",
        id: "comment-1",
        actor_type: "member",
        actor_id: "user-1",
        created_at: "2026-01-01T00:00:00Z",
        content: "still here",
        deleted_at: 42,
      },
    ]);

    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.deleted_at).toBeUndefined();
  });
});

describe("AgentTaskListSchema", () => {
  it("preserves negotiated supplement capability, ordered coverage and permission", () => {
    const parsed = AgentTaskListSchema.parse([{
      id: "run",
      supplement_capability: "task-supplement-v1",
      supplement_comment_ids: ["comment-1", "comment-2"],
      can_supplement: true,
    }]);
    expect(parsed[0]).toMatchObject({
      supplement_capability: "task-supplement-v1",
      supplement_comment_ids: ["comment-1", "comment-2"],
      can_supplement: true,
    });
  });

  it.each([true, false, undefined, null, "true", 1])("safely parses comment cancellation metadata: %s", (value) => {
    const parsed = AgentTaskListSchema.parse([{ id: "run", cancelled_by_comment_change: value }]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.cancelled_by_comment_change).toBe(typeof value === "boolean" ? value : undefined);
  });

  it("parses cancellation actor metadata without making it required", () => {
    const parsed = AgentTaskListSchema.parse([
      { id: "new", cancelled_by: { type: "member", id: "user-1", name: "Jiayuan" } },
      { id: "legacy" },
      { id: "malformed", cancelled_by: "member" },
    ]);

    expect(parsed[0]?.cancelled_by).toEqual({ type: "member", id: "user-1", name: "Jiayuan" });
    expect(parsed[1]?.cancelled_by).toBeUndefined();
    expect(parsed[2]?.cancelled_by).toBeUndefined();
  });

  const task = {
    id: "task-1",
    agent_id: "agent-1",
    runtime_id: "runtime-1",
    issue_id: "issue-1",
    status: "queued",
    priority: 0,
    dispatched_at: null,
    started_at: null,
    completed_at: null,
    result: null,
    error: null,
    created_at: "2026-07-10T00:00:00Z",
    trigger_comment_id: "comment-3",
  };

  it("preserves planned and delivered comment IDs for a task run", () => {
    const parsed = AgentTaskListSchema.parse([
      {
        ...task,
        coalesced_comment_ids: ["comment-1", "comment-2"],
        delivered_comment_ids: ["comment-1", "comment-2", "comment-3"],
      },
    ]);

    expect(parsed[0]?.trigger_comment_id).toBe("comment-3");
    expect(parsed[0]?.coalesced_comment_ids).toEqual([
      "comment-1",
      "comment-2",
    ]);
    expect(parsed[0]?.delivered_comment_ids).toEqual([
      "comment-1",
      "comment-2",
      "comment-3",
    ]);
  });

  it("accepts task payloads from older backends without comment coverage", () => {
    const parsed = AgentTaskListSchema.parse([task]);
    expect(parsed[0]?.coalesced_comment_ids).toBeUndefined();
    expect(parsed[0]?.delivered_comment_ids).toBeUndefined();
  });

  it("degrades malformed optional coverage without dropping task rows", () => {
    const parsed = AgentTaskListSchema.parse([
      {
        ...task,
        coalesced_comment_ids: ["comment-1", 2],
        delivered_comment_ids: "not-an-array",
      },
      {
        ...task,
        id: "task-2",
        delivered_comment_ids: ["comment-2", "comment-3"],
      },
    ]);

    expect(parsed).toHaveLength(2);
    expect(parsed[0]?.coalesced_comment_ids).toBeUndefined();
    expect(parsed[0]?.delivered_comment_ids).toBeUndefined();
    expect(parsed[1]?.delivered_comment_ids).toEqual([
      "comment-2",
      "comment-3",
    ]);
  });

  it("accepts durable workdir metadata from newer backends", () => {
    const parsed = AgentTaskListSchema.parse([
      {
        ...task,
        status: "completed",
        work_dir: "/managed/task/worktree",
        durable_work_dir: "/Users/dev/project",
        relative_durable_work_dir: "project",
        branch_name: "agent/j/abc12345",
      },
    ]);

    expect(parsed[0]).toMatchObject({
      durable_work_dir: "/Users/dev/project",
      relative_durable_work_dir: "project",
      branch_name: "agent/j/abc12345",
    });
  });

  it("degrades malformed optional path metadata without dropping task rows", () => {
    const parsed = AgentTaskListSchema.parse([
      {
        ...task,
        work_dir: 1,
        durable_work_dir: { path: "/project" },
        relative_durable_work_dir: false,
        branch_name: ["agent/j/abc12345"],
      },
    ]);

    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.work_dir).toBeUndefined();
    expect(parsed[0]?.durable_work_dir).toBeUndefined();
    expect(parsed[0]?.relative_durable_work_dir).toBeUndefined();
    expect(parsed[0]?.branch_name).toBeUndefined();
  });
});

describe("ChatDraftRestoresResponseSchema", () => {
  it("parses a well-formed response with attachments", () => {
    const parsed = parseWithFallback(
      {
        restores: [
          {
            id: "msg-1",
            chat_session_id: "s-1",
            task_id: "t-1",
            content: "run the thing",
            attachments: [{ id: "att-1", filename: "notes.txt" }],
            created_at: "2026-07-01T00:00:00Z",
          },
        ],
      },
      ChatDraftRestoresResponseSchema,
      EMPTY_CHAT_DRAFT_RESTORES,
      { endpoint: "test" },
    );
    expect(parsed.restores).toHaveLength(1);
    expect(parsed.restores[0]?.content).toBe("run the thing");
    expect(parsed.restores[0]?.attachments?.[0]?.id).toBe("att-1");
  });

  it("defaults a missing restores array instead of crashing the composer", () => {
    const parsed = parseWithFallback(
      {},
      ChatDraftRestoresResponseSchema,
      EMPTY_CHAT_DRAFT_RESTORES,
      { endpoint: "test" },
    );
    expect(parsed.restores).toEqual([]);
  });

  it("falls back to the empty response on a malformed row", () => {
    // A row without the consume key (id) is unusable — the whole response
    // falls back and the durable rows simply stay pending server-side.
    const parsed = parseWithFallback(
      { restores: [{ chat_session_id: "s-1", content: 42 }] },
      ChatDraftRestoresResponseSchema,
      EMPTY_CHAT_DRAFT_RESTORES,
      { endpoint: "test" },
    );
    expect(parsed).toEqual(EMPTY_CHAT_DRAFT_RESTORES);
  });
});

describe("ChatPendingTaskSchema", () => {
  const ENDPOINT = { endpoint: "GET /api/chat/sessions/:id/pending-task" };

  it("keeps legacy responses compatible when queued_tasks is absent", () => {
    const parsed = parseWithFallback(
      {
        task_id: "task-active",
        status: "running",
        created_at: "2026-07-01T00:00:00Z",
      },
      ChatPendingTaskSchema,
      EMPTY_CHAT_PENDING_TASK,
      ENDPOINT,
    );

    expect(parsed).toMatchObject({
      task_id: "task-active",
      status: "running",
    });
    expect(parsed.queued_tasks).toBeUndefined();
  });

  it("parses queued task summaries", () => {
    const parsed = ChatPendingTaskSchema.parse({
      task_id: "task-active",
      status: "running",
      queued_tasks: [
        {
          task_id: "task-queued",
          status: "queued",
          content: "Follow up after the current task",
          created_at: "2026-07-01T00:01:00Z",
        },
      ],
    });

    expect(parsed.queued_tasks).toEqual([
      expect.objectContaining({
        task_id: "task-queued",
        content: "Follow up after the current task",
      }),
    ]);
  });

  it("keeps a valid head and ignores only malformed queued rows", () => {
    const parsed = parseWithFallback(
      {
        task_id: "task-active",
        queued_tasks: [{ task_id: 42, status: "queued" }],
      },
      ChatPendingTaskSchema,
      EMPTY_CHAT_PENDING_TASK,
      ENDPOINT,
    );

    expect(parsed).toEqual({
      task_id: "task-active",
      queued_tasks: [],
    });
  });
});

describe("SendChatMessageResponseSchema", () => {
  const base = {
    message_id: "message-1",
    task_id: "task-1",
    created_at: "2026-08-05T00:00:00Z",
  };

  it("parses the server-authoritative queue position", () => {
    expect(SendChatMessageResponseSchema.parse({ ...base, queued: false }).queued).toBe(false);
  });

  it("ignores a malformed additive queue position without losing the accepted send", () => {
    expect(SendChatMessageResponseSchema.parse({ ...base, queued: "no" }).queued).toBeUndefined();
  });
});

describe("PrioritizeQueuedChatTaskResponseSchema", () => {
  const ENDPOINT = {
    endpoint: "POST /api/chat/sessions/:id/queued-tasks/:taskId/prioritize",
  };

  it("parses the prioritized task id", () => {
    expect(
      PrioritizeQueuedChatTaskResponseSchema.parse({
        task_id: "task-queued",
        active_task_id: "task-active",
      }),
    ).toEqual({
      task_id: "task-queued",
      active_task_id: "task-active",
    });
  });

  it("falls back when task_id is malformed", () => {
    expect(
      parseWithFallback(
        { task_id: 42 },
        PrioritizeQueuedChatTaskResponseSchema,
        EMPTY_PRIORITIZE_QUEUED_CHAT_TASK_RESPONSE,
        ENDPOINT,
      ),
    ).toBe(EMPTY_PRIORITIZE_QUEUED_CHAT_TASK_RESPONSE);
  });
});

describe("CreateFeedbackResponseSchema", () => {
  const ENDPOINT = { endpoint: "POST /api/feedback" };

  it("parses a well-formed response and preserves extra fields", () => {
    const parsed = parseWithFallback(
      { id: "feedback-1", created_at: "2026-06-26T00:00:00Z", future_field: true },
      CreateFeedbackResponseSchema,
      EMPTY_CREATE_FEEDBACK_RESPONSE,
      ENDPOINT,
    );
    expect(parsed).toMatchObject({
      id: "feedback-1",
      created_at: "2026-06-26T00:00:00Z",
      future_field: true,
    });
  });

  it("returns the empty fallback for malformed feedback responses", () => {
    expect(
      parseWithFallback(
        { id: 123, created_at: "2026-06-26T00:00:00Z" },
        CreateFeedbackResponseSchema,
        EMPTY_CREATE_FEEDBACK_RESPONSE,
        ENDPOINT,
      ),
    ).toBe(EMPTY_CREATE_FEEDBACK_RESPONSE);
    expect(
      parseWithFallback(null, CreateFeedbackResponseSchema, EMPTY_CREATE_FEEDBACK_RESPONSE, ENDPOINT),
    ).toBe(EMPTY_CREATE_FEEDBACK_RESPONSE);
  });
});

// The duplicate-issue branch in create-issue.tsx feeds ApiError.body
// (typed as `unknown`) through this schema. Any future server drift that
// loses the contract MUST fail the parse so the UI falls back to a normal
// error toast instead of rendering an empty / partial duplicate card.
describe("DuplicateIssueErrorBodySchema", () => {
  const valid = {
    code: "active_duplicate_issue",
    error: "An active issue with this title already exists: MUL-12 – Login bug",
    issue: {
      id: "11111111-1111-1111-1111-111111111111",
      identifier: "MUL-12",
      title: "Login bug",
    },
  };

  it("accepts a well-formed body", () => {
    expect(DuplicateIssueErrorBodySchema.safeParse(valid).success).toBe(true);
  });

  it("accepts unknown extra fields via .loose()", () => {
    const forwardCompat = {
      ...valid,
      hint: "Try a different title",
      issue: { ...valid.issue, workspace_id: "ws-1", status: "todo" },
    };
    expect(DuplicateIssueErrorBodySchema.safeParse(forwardCompat).success).toBe(true);
  });

  it("rejects a renamed code (so renames degrade to the generic toast)", () => {
    const renamed = { ...valid, code: "duplicate_issue" };
    expect(DuplicateIssueErrorBodySchema.safeParse(renamed).success).toBe(false);
  });

  it("rejects a missing issue object", () => {
    const { issue: _omit, ...without } = valid;
    expect(DuplicateIssueErrorBodySchema.safeParse(without).success).toBe(false);
  });

  it("rejects a non-string issue.id", () => {
    const broken = { ...valid, issue: { ...valid.issue, id: 42 } };
    expect(DuplicateIssueErrorBodySchema.safeParse(broken).success).toBe(false);
  });

  it("accepts a missing error field (it is optional)", () => {
    const { error: _omit, ...without } = valid;
    expect(DuplicateIssueErrorBodySchema.safeParse(without).success).toBe(true);
  });
});

// `user.timezone` (Viewing tz) was added in the timezone-architecture RFC.
// A desktop build older than the server — or a server predating the
// `user.timezone` migration — will return a `/api/me` body with no
// `timezone` key. The schema must not fail closed on that: the field
// defaults to `null`, which the frontend resolves to the browser-detected
// tz at render time.
describe("UserSchema timezone drift", () => {
  const base = {
    id: "11111111-1111-1111-1111-111111111111",
    name: "Ada",
    email: "ada@example.com",
  };

  it("defaults timezone to null when the field is absent", () => {
    const parsed = UserSchema.parse(base);
    expect(parsed.timezone).toBe(null);
  });

  it("preserves an explicit IANA timezone", () => {
    const parsed = UserSchema.parse({ ...base, timezone: "Asia/Tokyo" });
    expect(parsed.timezone).toBe("Asia/Tokyo");
  });

  it("accepts an explicit null timezone", () => {
    const parsed = UserSchema.parse({ ...base, timezone: null });
    expect(parsed.timezone).toBe(null);
  });

  // Wrong-type drift: a future server bug sending `timezone` as a number
  // must not throw into the UI. parseWithFallback degrades the whole user
  // object to the explicit fallback (EMPTY_USER) so /api/me callers keep a
  // valid shape instead of white-screening.
  it("falls back to EMPTY_USER when timezone is the wrong type", () => {
    const parsed = parseWithFallback(
      { ...base, timezone: 42 },
      UserSchema,
      EMPTY_USER,
      { endpoint: "GET /api/me" },
    );
    expect(parsed).toBe(EMPTY_USER);
  });
});

describe("SquadListSchema member preview drift", () => {
  const baseSquad = {
    id: "squad-1",
    workspace_id: "ws-1",
    name: "Frontend Squad",
    description: "",
    instructions: "",
    avatar_url: null,
    leader_id: "agent-1",
    creator_id: "user-1",
    created_at: "2026-05-01T00:00:00Z",
    updated_at: "2026-05-01T00:00:00Z",
    archived_at: null,
    archived_by: null,
  };

  it("defaults preview fields when an older backend omits them", () => {
    const parsed = SquadListSchema.parse([baseSquad]);
    expect(parsed[0]?.member_count).toBe(0);
    expect(parsed[0]?.member_preview).toEqual([]);
  });

  it("defaults preview fields on a single squad response", () => {
    const parsed = SquadSchema.parse(baseSquad);
    expect(parsed.member_count).toBe(0);
    expect(parsed.member_preview).toEqual([]);
  });

  it("preserves lightweight member preview rows", () => {
    const parsed = SquadListSchema.parse([
      {
        ...baseSquad,
        member_count: 2,
        member_preview: [
          { member_type: "agent", member_id: "agent-1", role: "leader" },
          { member_type: "member", member_id: "user-2", role: "member" },
        ],
      },
    ]);
    expect(parsed[0]?.member_count).toBe(2);
    expect(parsed[0]?.member_preview).toHaveLength(2);
    expect(parsed[0]?.member_preview?.[0]?.role).toBe("leader");
  });
});

describe("SquadSchema runtime skill drift", () => {
  const baseSquad = {
    id: "squad-1",
    workspace_id: "ws-1",
    name: "Frontend Squad",
    leader_id: "agent-1",
    creator_id: "user-1",
    created_at: "2026-05-01T00:00:00Z",
    updated_at: "2026-05-01T00:00:00Z",
  };

  it("gives no runtime skills when an older backend omits the section switch", () => {
    const parsed = SquadSchema.parse(baseSquad);
    expect(parsed.inherit_runtime_skills).toBe(false);
    expect(parsed.runtime_skill_exceptions).toEqual([]);
  });

  it("keeps the section switch and per-runtime exceptions", () => {
    const exception = {
      runtime_id: "runtime-1",
      provider: "omp",
      root: "provider",
      key: "review",
      name: "Review",
    };
    const parsed = SquadSchema.parse({
      ...baseSquad,
      inherit_runtime_skills: true,
      runtime_skill_exceptions: [exception],
    });
    expect(parsed.inherit_runtime_skills).toBe(true);
    expect(parsed.runtime_skill_exceptions).toEqual([exception]);
  });
});

describe("SquadMemberListSchema member type drift", () => {
  const row = (member_type: string, member_id: string) => ({
    id: `sm-${member_id}`,
    squad_id: "squad-1",
    member_type,
    member_id,
    role: "",
    created_at: "2026-05-01T00:00:00Z",
  });

  it("keeps agent, member and squad rows", () => {
    const parsed = SquadMemberListSchema.parse([
      row("agent", "agent-1"),
      row("member", "user-1"),
      row("squad", "squad-2"),
    ]);
    expect(parsed.map((m) => m.member_type)).toEqual(["agent", "member", "squad"]);
  });

  it("drops a row with a member type this client does not know", () => {
    const parsed = SquadMemberListSchema.parse([
      row("agent", "agent-1"),
      row("team", "team-1"),
    ]);
    expect(parsed.map((m) => m.member_id)).toEqual(["agent-1"]);
  });
});

// The workspace dashboard and runtime-detail pages were re-pointed at the
// unified `task_usage_hourly` rollup. Every numeric field drives chart /
// KPI math, and string keys (date / agent_id / model) bucket the series.
// The contract these schemas must hold: a row missing a field degrades
// that field to a sane default rather than dropping the WHOLE array to
// the `[]` fallback — one drifted row must not blank the entire chart.
describe("dashboard + runtime usage schema drift", () => {
  it("coerces a missing numeric field to 0 instead of dropping the array", () => {
    const parsed = DashboardUsageDailyListSchema.parse([
      { date: "2026-05-19", model: "claude-opus-4-7", input_tokens: 100 },
    ]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.output_tokens).toBe(0);
    expect(parsed[0]?.cache_read_tokens).toBe(0);
    expect(parsed[0]?.cache_write_tokens).toBe(0);
  });

  it("coerces a missing date key to \"\" so the rest of the series survives", () => {
    const parsed = DashboardUsageDailyListSchema.parse([
      { model: "claude-opus-4-7", input_tokens: 5 },
    ]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.date).toBe("");
  });

  it("coerces a missing agent_id key to \"\" for the agent-runtime panel", () => {
    const parsed = DashboardAgentRunTimeListSchema.parse([
      { total_seconds: 42, task_count: 3, failed_count: 0 },
    ]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.agent_id).toBe("");
  });

  it("defaults a missing cancelled_count to 0 so a pre-cancelled-count server still renders", () => {
    // cancelled_count was added when the run-time rollups started counting
    // runs the user stopped mid-flight. A backend predating it omits the
    // field; the row must survive with a 0 segment rather than drop the
    // whole series (installed desktop clients hit older backends).
    expect(
      DashboardAgentRunTimeListSchema.parse([
        { agent_id: "a", total_seconds: 42, task_count: 3, failed_count: 0 },
      ])[0]?.cancelled_count,
    ).toBe(0);
    expect(
      DashboardRunTimeDailyListSchema.parse([
        { date: "2026-05-19", total_seconds: 42, task_count: 3, failed_count: 0 },
      ])[0]?.cancelled_count,
    ).toBe(0);
  });

  it("preserves optional usage coverage without rejecting older or malformed rows", () => {
    const parsed = DashboardAgentRunTimeListSchema.parse([
      {
        agent_id: "new-server",
        total_seconds: 42,
        task_count: 3,
        metered_task_count: 2,
        failed_count: 0,
      },
      {
        agent_id: "old-server",
        total_seconds: 42,
        task_count: 3,
        failed_count: 0,
      },
      {
        agent_id: "drifted-server",
        total_seconds: 42,
        task_count: 3,
        metered_task_count: "not-a-number",
        failed_count: 0,
      },
    ]);
    expect(parsed[0]?.metered_task_count).toBe(2);
    expect(parsed[1]?.metered_task_count).toBeUndefined();
    expect(parsed[2]?.metered_task_count).toBeUndefined();
  });

  it("coerces a missing agent_id key to \"\" for the usage-by-agent panel", () => {
    const parsed = DashboardUsageByAgentListSchema.parse([
      { model: "claude-opus-4-7", input_tokens: 7 },
    ]);
    expect(parsed[0]?.agent_id).toBe("");
  });

  it("coerces missing fields on every runtime usage schema", () => {
    expect(RuntimeUsageListSchema.parse([{ date: "2026-05-19" }])[0]?.input_tokens).toBe(0);
    expect(RuntimeHourlyActivityListSchema.parse([{ hour: 9 }])[0]?.count).toBe(0);
    expect(RuntimeUsageByAgentListSchema.parse([{ model: "x" }])[0]?.agent_id).toBe("");
    expect(RuntimeUsageByHourListSchema.parse([{ hour: 9 }])[0]?.model).toBe("");
  });

  it("defaults a missing provider to \"\" so an older server's rows still price by bare model", () => {
    // provider was added for cross-provider model disambiguation; a server
    // predating it omits the field. The schema must fill "" (→ bare-model
    // pricing lookup) rather than drop the row.
    expect(
      DashboardUsageDailyListSchema.parse([{ date: "2026-05-19", model: "claude-opus-4-7" }])[0]
        ?.provider,
    ).toBe("");
    expect(
      DashboardUsageByAgentListSchema.parse([{ model: "claude-opus-4-7" }])[0]?.provider,
    ).toBe("");
    expect(RuntimeUsageByAgentListSchema.parse([{ model: "x" }])[0]?.provider).toBe("");
  });

  it("rejects a non-array body so parseWithFallback can return its fallback", () => {
    expect(DashboardUsageDailyListSchema.safeParse(null).success).toBe(false);
    expect(DashboardFailureDailyListSchema.safeParse(null).success).toBe(false);
    expect(DashboardFailureByAgentListSchema.safeParse({ rows: [] }).success).toBe(
      false,
    );
    expect(RuntimeUsageListSchema.safeParse({ rows: [] }).success).toBe(false);
  });

  it("keeps a failure_reason the client build has never heard of", () => {
    // failure_reason is an open string, not an enum: the backend taxonomy
    // grows, and an installed desktop client must still count a reason its
    // build predates rather than dropping the row (and with it the day's
    // error total).
    const parsed = DashboardFailureDailyListSchema.parse([
      { date: "2026-05-19", failure_reason: "agent_error.brand_new", task_count: 3 },
    ]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.failure_reason).toBe("agent_error.brand_new");
    expect(parsed[0]?.task_count).toBe(3);
  });

  it("coerces a missing failure row field without dropping the array", () => {
    const daily = DashboardFailureDailyListSchema.parse([{ date: "2026-05-19" }]);
    expect(daily).toHaveLength(1);
    // "" is the succeeded bucket, so a reason-less row lands in the
    // denominator instead of inventing a failure that never happened.
    //
    // Defaulting to a failure bucket instead was considered and rejected: the
    // realistic drift here is someone adding `omitempty` to the Go struct
    // tag, which would strip the field from exactly the SUCCESS rows and turn
    // every window into a 100% error rate. Deflating a rate under drift is
    // the milder failure. TestDashboardFailureWireContractKeepsEmptyReason
    // (server/internal/handler/dashboard_test.go) guards the other side by
    // pinning that the server always emits the field.
    expect(daily[0]?.failure_reason).toBe("");
    expect(daily[0]?.task_count).toBe(0);

    const byAgent = DashboardFailureByAgentListSchema.parse([
      { failure_reason: "timeout", task_count: 2 },
    ]);
    expect(byAgent[0]?.agent_id).toBe("");
  });

  it("keeps unknown server-side fields via .loose()", () => {
    const parsed = RuntimeUsageListSchema.parse([
      { date: "2026-05-19", region: "us-east" },
    ]);
    expect((parsed[0] as Record<string, unknown>).region).toBe("us-east");
  });
});

// A server that never heard of worktree mode also never sends this flag, and
// it does not reject the mode either — it drops execution_mode and answers 201,
// leaving the task to run in the user's working copy (#7113). So the absent
// case has to parse as false, not as "unknown, probably fine".
// An older server deletes a comment's replies with it and omits this field,
// so absent or malformed must parse as false: the client then promises nothing
// about replies and keeps the legacy delete route (#8296).
describe("AppConfigSchema comment_delete_keep_replies_supported drift", () => {
  it.each([
    [undefined, false],
    ["yes", false],
    [true, true],
  ])("%j parses as %s", (value, expected) => {
    const parsed = AppConfigSchema.parse({
      cdn_domain: "cdn.example.com",
      comment_delete_keep_replies_supported: value,
    });
    expect(parsed.comment_delete_keep_replies_supported).toBe(expected);
  });
});

describe("AppConfigSchema agent_conversation_starters_supported drift", () => {
  it("defaults to false when the server predates the persistence contract", () => {
    expect(AppConfigSchema.parse({}).agent_conversation_starters_supported).toBe(false);
  });

  it("coerces a malformed declaration to false", () => {
    expect(
      AppConfigSchema.parse({ agent_conversation_starters_supported: "yes" })
        .agent_conversation_starters_supported,
    ).toBe(false);
  });

  it("carries a genuine declaration through", () => {
    expect(
      AppConfigSchema.parse({ agent_conversation_starters_supported: true })
        .agent_conversation_starters_supported,
    ).toBe(true);
  });
});

describe("AppConfigSchema issue_create_properties_supported drift", () => {
  it("defaults to false when the server predates atomic create properties", () => {
    expect(AppConfigSchema.parse({}).issue_create_properties_supported).toBe(false);
  });

  it("coerces a malformed declaration to false", () => {
    expect(
      AppConfigSchema.parse({ issue_create_properties_supported: "yes" })
        .issue_create_properties_supported,
    ).toBe(false);
  });

  it("carries a genuine declaration through", () => {
    expect(
      AppConfigSchema.parse({ issue_create_properties_supported: true })
        .issue_create_properties_supported,
    ).toBe(true);
  });
});

describe("AppConfigSchema cdn_signed drift", () => {
  it("defaults cdn_signed to false when the server omits it (pre-MUL-3254 servers)", () => {
    const parsed = AppConfigSchema.parse({ cdn_domain: "cdn.example.com" });
    expect(parsed.cdn_signed).toBe(false);
  });

  it("coerces a malformed cdn_signed to false instead of failing the whole config", () => {
    const parsed = AppConfigSchema.parse({
      cdn_domain: "cdn.example.com",
      cdn_signed: "yes",
    });
    expect(parsed.cdn_signed).toBe(false);
    expect(parsed.cdn_domain).toBe("cdn.example.com");
  });

  it("keeps cdn_signed=true from a signing-enabled server", () => {
    const parsed = AppConfigSchema.parse({ cdn_signed: true });
    expect(parsed.cdn_signed).toBe(true);
  });

  it("parses frontend feature flag decisions", () => {
    const parsed = AppConfigSchema.parse({
      feature_flags: {
        composio_mcp_apps: true,
        malformed_future_flag: "yes",
      },
    });
    expect(parsed.feature_flags).toEqual({
      composio_mcp_apps: true,
      malformed_future_flag: false,
    });
  });

  it("defaults malformed feature_flags to an empty object", () => {
    const parsed = AppConfigSchema.parse({ feature_flags: ["not", "an", "object"] });
    expect(parsed.feature_flags).toEqual({});
  });

  it("parses server_version and leaves it undefined when the server omits it", () => {
    expect(AppConfigSchema.parse({ server_version: "1.2.3" }).server_version).toBe("1.2.3");
    expect(AppConfigSchema.parse({}).server_version).toBeUndefined();
  });
});

describe("InboxUnreadSummarySchema", () => {
  const ENDPOINT = { endpoint: "GET /api/inbox/unread-summary" };

  it("parses a well-formed summary and tolerates extra fields", () => {
    const parsed = parseWithFallback(
      [
        { workspace_id: "ws-1", count: 2 },
        { workspace_id: "ws-2", count: 0, future_field: "ignored" },
      ],
      InboxUnreadSummarySchema,
      EMPTY_INBOX_UNREAD_SUMMARY,
      ENDPOINT,
    );
    expect(parsed).toEqual([
      { workspace_id: "ws-1", count: 2 },
      { workspace_id: "ws-2", count: 0, future_field: "ignored" },
    ]);
  });

  it("returns the empty fallback (dot hidden) for a non-array body", () => {
    expect(
      parseWithFallback({ rows: [] }, InboxUnreadSummarySchema, EMPTY_INBOX_UNREAD_SUMMARY, ENDPOINT),
    ).toBe(EMPTY_INBOX_UNREAD_SUMMARY);
    expect(
      parseWithFallback(null, InboxUnreadSummarySchema, EMPTY_INBOX_UNREAD_SUMMARY, ENDPOINT),
    ).toBe(EMPTY_INBOX_UNREAD_SUMMARY);
  });

  it("returns the empty fallback when an entry has a wrong-typed count", () => {
    expect(
      parseWithFallback(
        [{ workspace_id: "ws-1", count: "lots" }],
        InboxUnreadSummarySchema,
        EMPTY_INBOX_UNREAD_SUMMARY,
        ENDPOINT,
      ),
    ).toBe(EMPTY_INBOX_UNREAD_SUMMARY);
  });
});

describe("InboxItemListSchema", () => {
  const ENDPOINT = { endpoint: "GET /api/inbox/archived" };

  const row = (overrides: Record<string, unknown> = {}) => ({
    id: "inbox-1",
    workspace_id: "ws-1",
    recipient_type: "member",
    recipient_id: "member-1",
    type: "new_comment",
    severity: "info",
    issue_id: "issue-1",
    title: "Issue title",
    body: null,
    read: false,
    archived: true,
    created_at: "2026-06-15T08:00:00Z",
    ...overrides,
  });

  it("parses a well-formed archived list and tolerates extra fields", () => {
    const parsed = parseWithFallback(
      [row({
        issue_status: "in_progress",
        issue_priority: "high",
        details: { comment_id: "c-1" },
        future_field: 1,
      })],
      InboxItemListSchema,
      EMPTY_INBOX_ITEMS,
      ENDPOINT,
    );
    expect(parsed).toHaveLength(1);
    expect(parsed[0]).toMatchObject({
      id: "inbox-1",
      archived: true,
      issue_status: "in_progress",
      issue_priority: "high",
    });
  });

  it("keeps a notification type this client doesn't know yet", () => {
    // Enums stay lenient on purpose: a backend that ships a new inbox type
    // must not blank the whole archived list on older clients.
    const parsed = parseWithFallback(
      [row({ type: "some_future_type", severity: "future_severity" })],
      InboxItemListSchema,
      EMPTY_INBOX_ITEMS,
      ENDPOINT,
    );
    expect(parsed).toHaveLength(1);
  });

  it("accepts rows that omit the nullable optional fields", () => {
    const { body, issue_id, ...withoutOptionals } = row();
    void body;
    void issue_id;
    expect(
      parseWithFallback([withoutOptionals], InboxItemListSchema, EMPTY_INBOX_ITEMS, ENDPOINT),
    ).toHaveLength(1);
  });

  it("returns the empty fallback when an issue projection is wrong-typed", () => {
    expect(
      parseWithFallback(
        [row({ issue_priority: 3 })],
        InboxItemListSchema,
        EMPTY_INBOX_ITEMS,
        ENDPOINT,
      ),
    ).toBe(EMPTY_INBOX_ITEMS);
  });

  it("returns the empty fallback for a non-array body", () => {
    expect(
      parseWithFallback({ items: [] }, InboxItemListSchema, EMPTY_INBOX_ITEMS, ENDPOINT),
    ).toBe(EMPTY_INBOX_ITEMS);
    expect(
      parseWithFallback(null, InboxItemListSchema, EMPTY_INBOX_ITEMS, ENDPOINT),
    ).toBe(EMPTY_INBOX_ITEMS);
  });

  it("returns the empty fallback when a row is missing a required field", () => {
    const { id, ...withoutId } = row();
    void id;
    expect(
      parseWithFallback([withoutId], InboxItemListSchema, EMPTY_INBOX_ITEMS, ENDPOINT),
    ).toBe(EMPTY_INBOX_ITEMS);
  });

  it("returns the empty fallback when `archived` is wrong-typed", () => {
    expect(
      parseWithFallback(
        [row({ archived: "yes" })],
        InboxItemListSchema,
        EMPTY_INBOX_ITEMS,
        ENDPOINT,
      ),
    ).toBe(EMPTY_INBOX_ITEMS);
  });
});

describe("SearchProjectsResponseSchema drift", () => {
  const ENDPOINT = { endpoint: "GET /api/projects/search" };

  const baseProject = {
    id: "p-1",
    workspace_id: "ws-1",
    title: "Launch",
    description: null,
    icon: null,
    status: "in_progress",
    priority: "high",
    lead_type: null,
    lead_id: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    issue_count: 0,
    done_count: 0,
    match_source: "title",
  };

  it("parses start_date / due_date when the backend returns them", () => {
    const parsed = parseWithFallback(
      { projects: [{ ...baseProject, start_date: "2026-03-01", due_date: "2026-03-31" }], total: 1 },
      SearchProjectsResponseSchema,
      EMPTY_SEARCH_PROJECTS_RESPONSE,
      ENDPOINT,
    );
    expect(parsed.projects[0]?.start_date).toBe("2026-03-01");
    expect(parsed.projects[0]?.due_date).toBe("2026-03-31");
  });

  // Frontend deploys before backend: an older backend omits the new keys. The
  // .default(null) must keep the whole batch parseable (→ null), not degrade
  // it to the empty fallback and blank the search results.
  it("defaults missing start_date / due_date to null without dropping results", () => {
    const parsed = parseWithFallback(
      { projects: [baseProject], total: 1 },
      SearchProjectsResponseSchema,
      EMPTY_SEARCH_PROJECTS_RESPONSE,
      ENDPOINT,
    );
    expect(parsed).not.toBe(EMPTY_SEARCH_PROJECTS_RESPONSE);
    expect(parsed.projects).toHaveLength(1);
    expect(parsed.projects[0]?.start_date).toBeNull();
    expect(parsed.projects[0]?.due_date).toBeNull();
  });

  it("parses organisation_id when the backend returns it", () => {
    const parsed = parseWithFallback(
      { projects: [{ ...baseProject, organisation_id: "org-1" }], total: 1 },
      SearchProjectsResponseSchema,
      EMPTY_SEARCH_PROJECTS_RESPONSE,
      ENDPOINT,
    );
    expect(parsed.projects[0]?.organisation_id).toBe("org-1");
  });

  // A backend before organisations omits the key: the project parses as having
  // no organisation instead of blanking the search results.
  it("defaults a missing organisation_id to null without dropping results", () => {
    const parsed = parseWithFallback(
      { projects: [baseProject], total: 1 },
      SearchProjectsResponseSchema,
      EMPTY_SEARCH_PROJECTS_RESPONSE,
      ENDPOINT,
    );
    expect(parsed).not.toBe(EMPTY_SEARCH_PROJECTS_RESPONSE);
    expect(parsed.projects).toHaveLength(1);
    expect(parsed.projects[0]?.organisation_id).toBeNull();
  });
});

// Organisations group the Projects page and back their pages, so drift must
// never blank every organisation for one bad row (projects-by-organisation).
describe("organisation schemas", () => {
  const LIST_ENDPOINT = { endpoint: "GET /api/organisations" };
  const DETAIL_ENDPOINT = { endpoint: "GET /api/organisations/{id}" };

  const baseOrganisation = {
    id: "org-1",
    workspace_id: "ws-1",
    name: "Rebl Records",
    key: "rebl",
    created_at: "2026-10-13T00:00:00Z",
    updated_at: "2026-10-13T00:00:00Z",
  };

  it("parses a well-formed list and keeps extra fields", () => {
    const parsed = parseWithFallback(
      { organisations: [{ ...baseOrganisation, future_field: true }], total: 1 },
      ListOrganisationsResponseSchema,
      EMPTY_LIST_ORGANISATIONS_RESPONSE,
      LIST_ENDPOINT,
    );
    expect(parsed.total).toBe(1);
    expect(parsed.organisations).toEqual([{ ...baseOrganisation, future_field: true }]);
  });

  it("defaults a missing organisations array to an empty list", () => {
    const parsed = parseWithFallback(
      { total: 3 },
      ListOrganisationsResponseSchema,
      EMPTY_LIST_ORGANISATIONS_RESPONSE,
      LIST_ENDPOINT,
    );
    expect(parsed.organisations).toEqual([]);
    expect(parsed.total).toBe(3);
    expect(ListOrganisationsResponseSchema.parse({})).toEqual({ organisations: [], total: 0 });
  });

  it("falls back to the empty list when the body is not an object", () => {
    for (const malformed of [null, "nope", 42, { organisations: "nope" }]) {
      expect(
        parseWithFallback(
          malformed,
          ListOrganisationsResponseSchema,
          EMPTY_LIST_ORGANISATIONS_RESPONSE,
          LIST_ENDPOINT,
        ),
      ).toEqual(EMPTY_LIST_ORGANISATIONS_RESPONSE);
    }
  });

  it("drops an organisation without key without hiding the others", () => {
    const { key: _key, ...withoutKey } = { ...baseOrganisation, id: "org-2", name: "Buttler" };
    const parsed = parseWithFallback(
      { organisations: [baseOrganisation, withoutKey, { ...baseOrganisation, id: "org-3", key: 42 }], total: 3 },
      ListOrganisationsResponseSchema,
      EMPTY_LIST_ORGANISATIONS_RESPONSE,
      LIST_ENDPOINT,
    );
    expect(parsed.organisations.map((organisation) => organisation.id)).toEqual(["org-1"]);
  });

  it("falls back to EMPTY_ORGANISATION for a single organisation without key", () => {
    const { key: _key, ...withoutKey } = baseOrganisation;
    expect(
      parseWithFallback(withoutKey, OrganisationSchema, EMPTY_ORGANISATION, DETAIL_ENDPOINT),
    ).toEqual(EMPTY_ORGANISATION);
    expect(EMPTY_ORGANISATION.id).toBe("");
  });

  it("defaults missing timestamps without dropping the organisation", () => {
    const { created_at: _createdAt, updated_at: _updatedAt, ...withoutTimestamps } = baseOrganisation;
    const parsed = parseWithFallback(
      withoutTimestamps,
      OrganisationSchema,
      EMPTY_ORGANISATION,
      DETAIL_ENDPOINT,
    );
    expect(parsed).toEqual({ ...withoutTimestamps, created_at: "", updated_at: "" });
  });
});

// The "run now" flow branches on run.status/reason_code to avoid a false-success
// toast (MUL-4525), so the trigger response must survive backend drift.
describe("AutopilotRunSchema", () => {
  const ENDPOINT = { endpoint: "POST /api/autopilots/:id/trigger" };
  const baseRun = {
    id: "run-1",
    autopilot_id: "ap-1",
    trigger_id: null,
    source: "manual",
    status: "issue_created",
    issue_id: "issue-1",
    task_id: null,
    triggered_at: "2026-07-14T00:00:00Z",
    completed_at: null,
    failure_reason: null,
    trigger_payload: null,
    result: null,
    created_at: "2026-07-14T00:00:00Z",
  };

  it("preserves a blocked run's status and reason_code", () => {
    const parsed = parseWithFallback(
      { ...baseRun, status: "skipped", failure_reason: "you are not allowed to trigger this autopilot's assignee agent", reason_code: "invocation_not_allowed" },
      AutopilotRunSchema,
      FALLBACK_AUTOPILOT_RUN,
      ENDPOINT,
    );
    expect(parsed.status).toBe("skipped");
    expect(parsed.reason_code).toBe("invocation_not_allowed");
  });

  it("tolerates an older server omitting reason_code", () => {
    const parsed = parseWithFallback(baseRun, AutopilotRunSchema, FALLBACK_AUTOPILOT_RUN, ENDPOINT);
    expect(parsed.status).toBe("issue_created");
    expect(parsed.reason_code).toBeUndefined();
  });

  it("degrades a malformed response to a non-success fallback (never a false success)", () => {
    const parsed = parseWithFallback("not-an-object", AutopilotRunSchema, FALLBACK_AUTOPILOT_RUN, ENDPOINT);
    expect(parsed).toBe(FALLBACK_AUTOPILOT_RUN);
    expect(parsed.status).toBe("failed");
  });
});

describe("AutopilotQuotaUsageSchema", () => {
  const baseUsage = {
    action: "enforce",
    used: 12,
    reserved: 2,
    limit: 100,
    period_start: "2026-08-01T00:00:00Z",
    period_end: "2026-09-01T00:00:00Z",
    reset_at: "2026-09-01T00:00:00Z",
  };

  it("preserves durable blocked counts by execution source", () => {
    const parsed = AutopilotQuotaUsageSchema.parse({
      ...baseUsage,
      blocked_counts: { schedule: 3, webhook: 7 },
    });
    expect(parsed.blocked_counts).toEqual({ schedule: 3, webhook: 7 });
  });

  it("defaults blocked_counts to null for an older server", () => {
    expect(AutopilotQuotaUsageSchema.parse(baseUsage).blocked_counts).toBeNull();
  });

  it("isolates a malformed blocked_counts field", () => {
    const parsed = AutopilotQuotaUsageSchema.parse({
      ...baseUsage,
      blocked_counts: { webhook: "many" },
    });
    expect(parsed.used).toBe(12);
    expect(parsed.blocked_counts).toBeNull();
  });
});

// The comment composer branches on preview.blocked to warn before sending
// (MUL-4525 §2), so the additive field must parse and degrade gracefully.
describe("CommentTriggerPreviewSchema.blocked", () => {
  it("parses blocked mention outcomes alongside agents", () => {
    const parsed = CommentTriggerPreviewSchema.parse({
      agents: [{ id: "a1", source: "mention_agent", reason: "" }],
      blocked: [
        { target_type: "squad", target_id: "s1", status: "blocked", reason_code: "invocation_not_allowed" },
      ],
    });
    expect(parsed.agents).toHaveLength(1);
    expect(parsed.blocked).toEqual([
      { target_type: "squad", target_id: "s1", status: "blocked", reason_code: "invocation_not_allowed" },
    ]);
  });

  it("defaults blocked to [] when an older server omits it", () => {
    const parsed = CommentTriggerPreviewSchema.parse({ agents: [] });
    expect(parsed.blocked).toEqual([]);
  });

  it("degrades a malformed blocked field to [] without dropping agents", () => {
    const parsed = CommentTriggerPreviewSchema.parse({
      agents: [{ id: "a1", source: "mention_agent", reason: "" }],
      blocked: "nope",
    });
    expect(parsed.agents).toHaveLength(1);
    expect(parsed.blocked).toEqual([]);
  });

  it("drops a single malformed blocked entry without discarding the valid ones", () => {
    const parsed = CommentTriggerPreviewSchema.parse({
      agents: [],
      blocked: [
        { target_type: "squad", target_id: "s1", status: "blocked", reason_code: "invocation_not_allowed" },
        { status: "blocked" }, // missing target_id → dropped individually
        { target_type: "agent", target_id: "a1", status: "blocked", reason_code: "runtime_offline" },
      ],
    });
    expect(parsed.blocked.map((b) => b.target_id)).toEqual(["s1", "a1"]);
  });
});

describe("RuntimeModelListRequestSchema", () => {
  const completed = {
    id: "req-1",
    runtime_id: "rt-1",
    status: "completed",
    supported: true,
    created_at: "2026-07-29T00:00:00Z",
    updated_at: "2026-07-29T00:00:01Z",
    models: [
      {
        id: "gpt-5.6-sol",
        label: "GPT-5.6-Sol",
        provider: "openai",
        default: true,
        thinking: {
          supported_levels: [{ value: "high", label: "High" }],
          default_level: "low",
        },
        service_tiers: [{ id: "fast", name: "Fast" }],
        supports_explicit_standard_service_tier: true,
      },
    ],
  };

  it("parses a live completed discovery, keeping the fields the UI branches on", () => {
    const parsed = parseWithFallback(
      completed,
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect(parsed.status).toBe("completed");
    expect(parsed.supported).toBe(true);
    expect(parsed.models?.[0]?.default).toBe(true);
    expect(parsed.models?.[0]?.thinking?.supported_levels).toEqual([
      { value: "high", label: "High" },
    ]);
    expect(parsed.models?.[0]?.service_tiers).toEqual([{ id: "fast", name: "Fast" }]);
    expect(
      parsed.models?.[0]?.supports_explicit_standard_service_tier,
    ).toBe(true);
    expect(parsed.cached).toBeUndefined();
  });

  it("keeps the additive cache markers when the server serves a snapshot", () => {
    const parsed = parseWithFallback(
      { ...completed, cached: true, cached_at: "2026-07-29T00:00:00Z" },
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect(parsed.cached).toBe(true);
    expect(parsed.cached_at).toBe("2026-07-29T00:00:00Z");
  });

  // A backend that predates MUL-5444 sends neither marker; an even older one
  // may omit `supported`. Both must stay usable rather than reading as
  // "runtime manages the model itself" off an undefined.
  it("defaults supported to true on an older backend that omits it", () => {
    const { supported: _omitted, ...withoutSupported } = completed;
    const parsed = parseWithFallback(
      withoutSupported,
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect(parsed.supported).toBe(true);
    expect(parsed.cached).toBeUndefined();
  });

  // MUL-6961: models the runtime cannot run arrive in their OWN list. The
  // separation is the compatibility contract — a client that never learned the
  // field reads `models` and therefore cannot offer one — so the schema must
  // keep them apart rather than folding them together.
  it("parses unavailable models without letting them into the selectable list", () => {
    const parsed = parseWithFallback(
      {
        ...completed,
        unavailable_models: [
          {
            id: "cc-update-required-1",
            label: "Fable 5.1 (disabled)",
            reason: "Update to 2.1.255+ to use Fable 5.1",
          },
        ],
      },
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect(parsed.unavailable_models?.[0]?.id).toBe("cc-update-required-1");
    expect(parsed.unavailable_models?.[0]?.reason).toBe(
      "Update to 2.1.255+ to use Fable 5.1",
    );
    expect(parsed.models?.map((m) => m.id)).not.toContain(
      "cc-update-required-1",
    );
  });

  // An older daemon or server sends no such key. The picker then shows no
  // unavailable section, which is exactly the pre-MUL-6961 behaviour.
  it("tolerates a backend that omits unavailable models entirely", () => {
    const parsed = parseWithFallback(
      completed,
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect(parsed.unavailable_models).toBeUndefined();
    expect(parsed.models?.length).toBe(1);
  });

  it("treats an older daemon that omits explicit-standard support as unsupported", () => {
    const model = completed.models[0]!;
    const {
      supports_explicit_standard_service_tier: _omitted,
      ...oldDaemonModel
    } = model;
    const parsed = parseWithFallback(
      { ...completed, models: [oldDaemonModel] },
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );

    expect(
      parsed.models?.[0]?.supports_explicit_standard_service_tier,
    ).toBeUndefined();
  });

  it("passes an unknown status through instead of failing the whole response", () => {
    const parsed = parseWithFallback(
      { ...completed, status: "superseded" },
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect(parsed.status).toBe("superseded");
  });

  // Malformed bodies must land on the "failed" fallback: `completed` would
  // fabricate an empty catalog and `pending` would spin the picker until the
  // client-side poll timeout.
  it("falls back to an explicit failure on a malformed body", () => {
    for (const malformed of [
      null,
      "nope",
      42,
      {},
      { status: 7 },
      { ...completed, status: undefined },
      { ...completed, supported: "yes" },
      { ...completed, models: "nope" },
      { ...completed, models: [{ label: "no id" }] },
      {
        ...completed,
        models: [
          {
            ...completed.models[0],
            supports_explicit_standard_service_tier: "yes",
          },
        ],
      },
    ]) {
      const parsed = parseWithFallback(
        malformed,
        RuntimeModelListRequestSchema,
        MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
        { endpoint: "test" },
      );
      expect(parsed.status).toBe("failed");
      expect(parsed.supported).toBe(true);
      expect(parsed.error).toBe("invalid model discovery response");
    }
  });

  it("keeps unknown server fields instead of stripping them", () => {
    const parsed = parseWithFallback(
      { ...completed, future_field: "keep me" },
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect((parsed as unknown as { future_field?: string }).future_field).toBe(
      "keep me",
    );
  });
});

describe("RuntimeDirectoryListingRequestSchema", () => {
  const completed = {
    id: "listing-1",
    runtime_id: "runtime-1",
    status: "completed",
    path: "/skills",
    entries: [
      { name: ".hidden", path: "/skills/.hidden", kind: "folder", hidden: true },
      { name: "SKILL.md", path: "/skills/SKILL.md", kind: "file", hidden: false },
    ],
    error: "",
    created_at: "2026-10-13T00:00:00Z",
    updated_at: "2026-10-13T00:00:00Z",
  };

  function parse(raw: unknown) {
    return parseWithFallback(
      raw,
      RuntimeDirectoryListingRequestSchema,
      MALFORMED_RUNTIME_DIRECTORY_LISTING_REQUEST,
      { endpoint: "test" },
    );
  }

  it("preserves a completed listing and its ordered file and folder entries", () => {
    expect(parse(completed)).toEqual(completed);
  });

  it("turns an unknown status into a failure without losing the server error", () => {
    const parsed = parse({ ...completed, status: "superseded", error: "expired" });
    expect(parsed.status).toBe("failed");
    expect(parsed.error).toBe("expired");
  });

  it("defaults missing entries to an empty list", () => {
    const { entries: _entries, ...withoutEntries } = completed;
    expect(parse(withoutEntries).entries).toEqual([]);
  });

  it("defaults missing entry kind and hidden markers", () => {
    const parsed = parse({
      ...completed,
      entries: [{ name: "SKILL.md", path: "/skills/SKILL.md" }],
    });
    expect(parsed.entries).toEqual([
      { name: "SKILL.md", path: "/skills/SKILL.md", kind: "file", hidden: false },
    ]);
  });

  it("defaults malformed entry fields without discarding other entries", () => {
    const parsed = parse({
      ...completed,
      entries: [
        { name: 42, path: null, kind: "symlink", hidden: "yes" },
        completed.entries[0],
      ],
    });
    expect(parsed.entries).toEqual([
      { name: "", path: "", kind: "file", hidden: false },
      completed.entries[0],
    ]);
  });

  it("defaults missing and malformed strings and entries", () => {
    expect(parse({ status: "completed", path: 42, entries: null })).toEqual({
      id: "",
      runtime_id: "",
      status: "completed",
      path: "",
      entries: [],
      error: "",
      created_at: "",
      updated_at: "",
    });
  });

  it("falls back to a failed request for a non-object response", () => {
    expect(parse(null)).toEqual(MALFORMED_RUNTIME_DIRECTORY_LISTING_REQUEST);
  });
});

describe("IssueViewSchema", () => {
  const valid = {
    id: "v1",
    workspace_id: "ws1",
    owner_id: "u1",
    name: "Needs review",
    scope_type: "workspace",
    scope_id: null,
    scope_variant: null,
    visibility: "workspace",
    definition_version: 1,
    query: { statusFilters: ["in_review"] },
    display: { viewMode: "board" },
    revision: 3,
    created_at: "2026-08-06T00:00:00Z",
    updated_at: "2026-08-06T00:00:00Z",
  };

  it("parses a well-formed view and keeps unknown future fields", () => {
    const parsed = IssueViewSchema.parse({ ...valid, future_field: "keep me" });
    expect(parsed.name).toBe("Needs review");
    expect(parsed.query).toEqual({ statusFilters: ["in_review"] });
    expect((parsed as unknown as { future_field?: string }).future_field).toBe("keep me");
  });

  it("defaults missing definition blobs instead of failing", () => {
    const parsed = IssueViewSchema.parse({ id: "v2" });
    expect(parsed.query).toEqual({});
    expect(parsed.display).toEqual({});
    expect(parsed.revision).toBe(1);
  });

  it("degrades a malformed list response to [] via parseWithFallback", () => {
    expect(
      parseWithFallback({ nonsense: true }, IssueViewListSchema, [], {
        endpoint: "GET /api/issue-views",
      }),
    ).toEqual([]);
    expect(
      parseWithFallback(null, IssueViewListSchema, [], {
        endpoint: "GET /api/issue-views",
      }),
    ).toEqual([]);
  });

  it("degrades a malformed detail response to null — NOT an error", () => {
    // The sidebar's pinned view rows hinge on this distinction: a parse
    // fallback (null, no error) hides the row, while only a REAL 404
    // error may ever unpin. A malformed body must never destroy a pin.
    expect(
      parseWithFallback({ nonsense: true }, IssueViewSchema.nullable(), null, {
        endpoint: "GET /api/issue-views/{id}",
      }),
    ).toBeNull();
  });
});

describe("Plugin schemas", () => {
  it("defaults every missing installation field to an inert, disabled shape", () => {
    const parsed = PluginInstallationSchema.parse({ id: "installation-1" });
    expect(parsed.enabled).toBe(false);
    expect(parsed.granted_scopes).toEqual([]);
    expect(parsed.config_schema).toEqual([]);
    expect(parsed.configured_secrets).toEqual([]);
    expect(parsed.surfaces).toEqual([]);
    expect(parsed.hooks).toEqual([]);
    expect(parsed.resources).toEqual([]);
  });

  it("does not model a secret value even when the server sends one", () => {
    // The API contract is that a secret is write-only. If a future response
    // ever regressed and echoed one, the client must not carry it into typed
    // state where a component could render it.
    const parsed = PluginInstallationSchema.parse({
      id: "installation-1",
      config: { repo: "multica-ai/multica" },
      configured_secrets: ["token"],
    });
    expect(parsed.config).toEqual({ repo: "multica-ai/multica" });
    expect(parsed.configured_secrets).toEqual(["token"]);
    expect(Object.keys(parsed)).not.toContain("secrets");
  });

  it("keeps config field declaration order so the generated form is stable", () => {
    const parsed = PluginInstallationSchema.parse({
      id: "installation-1",
      config_schema: [
        { key: "repo", type: "string", label: "Repo", required: true },
        { key: "token", type: "secret", label: "Token" },
      ],
    });
    expect(parsed.config_schema.map((field) => field.key)).toEqual(["repo", "token"]);
    expect(parsed.config_schema[1]?.required).toBe(false);
  });

  it("degrades a malformed installation list to empty rather than a partial list", () => {
    const parsed = parseWithFallback("not-json", PluginInstallationListResponseSchema, EMPTY_PLUGIN_INSTALLATION_LIST, {
      endpoint: "GET /api/workspaces/{id}/plugins",
    });
    expect(parsed).toEqual(EMPTY_PLUGIN_INSTALLATION_LIST);
  });

  it("degrades a malformed preview so the consent screen cannot show a blank scope list as approval", () => {
    const parsed = parseWithFallback({ scopes: "issues:read" }, PluginPreviewSchema, EMPTY_PLUGIN_PREVIEW, {
      endpoint: "POST /api/workspaces/{id}/plugins/preview",
    });
    expect(parsed).toEqual(EMPTY_PLUGIN_PREVIEW);
    expect(parsed.scopes).toEqual([]);
  });

  it("degrades a malformed MCP tool list to empty rather than showing tools as approved", () => {
    const parsed = parseWithFallback({ tools: "search" }, PluginMCPToolListSchema, { tools: [] }, {
      endpoint: "GET /api/workspaces/{id}/plugins/{installationId}/mcp/{hookKey}/tools",
    });
    expect(parsed.tools).toEqual([]);
  });

  // The dangerous direction is one-sided: a response missing `approved` must
  // read as NOT approved. The opposite default would render an unpinned tool
  // with a checked box, and the administrator's next save would pin it.
  it("treats a tool with no approval field as unapproved", () => {
    const parsed = PluginMCPToolListSchema.parse({ tools: [{ name: "search" }] });
    expect(parsed.tools[0]?.approved).toBe(false);
    expect(parsed.tools[0]?.drifted).toBe(false);
  });

  it("parses a preview that reports an upgrade adding new scopes", () => {
    const parsed = PluginPreviewSchema.parse({
      manifest: { key: "com.example.hello", name: "Hello", version: "2.0.0", author: { name: "example" } },
      scopes: ["issues:read", "comments:write"],
      installed: true,
      installed_version: "1.0.0",
      added_scopes: ["comments:write"],
    });
    expect(parsed.installed).toBe(true);
    expect(parsed.installed_version).toBe("1.0.0");
    expect(parsed.added_scopes).toEqual(["comments:write"]);
  });

  it("preserves automatic schedules on both consent and installed Plugin payloads", () => {
    const schedule = { cron: "*/5 * * * *", timezone: "Asia/Shanghai" };
    const preview = PluginPreviewSchema.parse({
      manifest: {
        key: "com.example.digest",
        name: "Digest",
        version: "1.0.0",
        author: { name: "example" },
        contributes: {
          hooks: [{ key: "digest", name: "Digest", triggers: ["schedule"], schedule }],
        },
      },
    });
    expect(preview.manifest.contributes?.hooks?.[0]?.schedule).toEqual(schedule);

    const installation = PluginInstallationSchema.parse({
      id: "installation-1",
      hooks: [{
        key: "digest",
        name: "Digest",
        triggers: ["schedule"],
        schedule: { ...schedule, next_run_at: "2026-08-23T10:15:00Z" },
      }],
    });
    expect(installation.hooks[0]?.schedule?.next_run_at).toBe("2026-08-23T10:15:00Z");
  });
});

// Issue status catalog (MUL-6243). The catalog drives how every status renders,
// so a drifting or malformed response must degrade to the built-ins rather than
// leaving the UI with no statuses at all.
describe("issue status catalog schemas", () => {
  const baseStatus = {
    id: "status-1",
    workspace_id: "ws-1",
    key: "human_review",
    name: "Human Review",
    description: "Waiting on a person",
    category: "in_review",
    color: "#22c55e",
    is_system: false,
    position: 1,
    archived_at: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };

  it("parses a full catalog response", () => {
    const parsed = ListIssueStatusesResponseSchema.parse({
      statuses: [baseStatus],
      categories: ["backlog", "todo", "in_progress", "in_review", "blocked", "done", "cancelled"],
      total: 1,
    });
    expect(parsed.statuses[0]?.key).toBe("human_review");
    expect(parsed.statuses[0]?.category).toBe("started");
    expect(parsed.categories).toHaveLength(4);
  });

  it("falls back to the built-in categories on a malformed response", () => {
    const parsed = parseWithFallback(
      { statuses: "not-an-array", categories: 7 },
      ListIssueStatusesResponseSchema,
      EMPTY_LIST_ISSUE_STATUSES_RESPONSE,
      { endpoint: "GET /api/issue-statuses" },
    );
    expect(parsed).toEqual(EMPTY_LIST_ISSUE_STATUSES_RESPONSE);
    // The fallback still names all 5 lifecycle categories, so a malformed
    // response cannot leave grouped issue surfaces without columns.
    expect(parsed.categories).toHaveLength(4);
    expect(parsed.statuses).toEqual([]);
  });

  it("defaults the optional presentation fields the server may omit", () => {
    const { color: _c, is_system: _s, position: _p, description: _d, archived_at: _a, ...minimal } = baseStatus;
    const parsed = IssueStatusEntrySchema.parse(minimal);
    expect(parsed.color).toBe("#6b7280");
    expect(parsed.is_system).toBe(false);
    expect(parsed.position).toBe(0);
    expect(parsed.archived_at).toBeNull();
  });

  it.each([undefined, null, "", "three_quarters", "future-icon"])("keeps catalog readable with icon %s", (icon) => {
    const parsed = IssueStatusEntrySchema.parse({ ...baseStatus, icon });
    expect(parsed.key).toBe(baseStatus.key);
    expect(parsed.icon).toBe(icon);
  });

  // PATCH /api/issue-statuses/reorder returns the same catalog shape as the
  // list endpoint, so a malformed reorder response degrades the same way rather
  // than leaving the settings page holding an unparsed blob. (MUL-6243)
  it("falls back on a malformed reorder response", () => {
    const parsed = parseWithFallback(
      { statuses: [{ id: 1 }], total: "many" },
      ListIssueStatusesResponseSchema,
      EMPTY_LIST_ISSUE_STATUSES_RESPONSE,
      { endpoint: "PATCH /api/issue-statuses/reorder" },
    );
    expect(parsed).toEqual(EMPTY_LIST_ISSUE_STATUSES_RESPONSE);
  });

  it("keeps an unknown category as a string instead of failing the whole catalog", () => {
    // A newer server could report a category this build does not know. Dropping
    // the entry (or throwing) would leave the board unable to render issues
    // already sitting on it, so the value is carried through verbatim.
    const parsed = ListIssueStatusesResponseSchema.parse({
      statuses: [{ ...baseStatus, category: "started" }],
      categories: ["started"],
      total: 1,
    });
    expect(parsed.statuses[0]?.category).toBe("started");
  });

  it("falls back to an empty entry on a malformed single status", () => {
    const parsed = parseWithFallback(
      { id: 12345 },
      IssueStatusEntrySchema,
      EMPTY_ISSUE_STATUS_ENTRY,
      { endpoint: "POST /api/issue-statuses" },
    );
    expect(parsed).toEqual(EMPTY_ISSUE_STATUS_ENTRY);
  });
});

describe("TaskMessageListSchema", () => {
  it("preserves call IDs and tolerates old or malformed optional identity", () => {
    const base = { task_id: "task-1", seq: 1, type: "tool_result", output: "ok" };
    const parsed = parseWithFallback<{ call_id?: string; output?: string }[]>(
      [
        { ...base, call_id: "execution:A" },
        base,
        { ...base, call_id: null },
        { ...base, call_id: 42 },
        { ...base, call_id: {} },
      ],
      TaskMessageListSchema, [], { endpoint: "GET /api/tasks/:id/messages" },
    );
    expect(parsed).toHaveLength(5);
    expect(parsed.map((m) => m.call_id)).toEqual(["execution:A", undefined, undefined, undefined, undefined]);
    expect(parsed.every((m) => m.output === "ok")).toBe(true);
  });

  const row = { task_id: "task-1", issue_id: "issue-1", seq: 1, type: "tool_result", output: "log line" };

  // The whole point of the field: a server that never sends it is saying
  // "nobody measured this", and only `undefined` can carry that. A default of
  // false would make every historical row assert it is complete.
  it("leaves a missing truncation flag undefined rather than false", () => {
    const parsed = TaskMessageListSchema.parse([row]);
    expect(parsed[0]).not.toHaveProperty("output_truncated", false);
    expect(parsed[0]?.output_truncated).toBeUndefined();
  });

  it("keeps both measured values", () => {
    const parsed = TaskMessageListSchema.parse([
      { ...row, seq: 1, output_truncated: true },
      { ...row, seq: 2, output_truncated: false },
    ]);
    expect(parsed.map((m) => m.output_truncated)).toEqual([true, false]);
  });

  // Drift defense. Without a field-level catch, one bad boolean fails its row,
  // the array fails with it, and parseWithFallback hands the viewer an empty
  // transcript — a malformed flag would delete the whole run from the screen.
  // Degrading the field to "unknown" is the correct loss.
  it("keeps the record and forgets the field when the flag is malformed", () => {
    const parsed = parseWithFallback<{ output?: string; output_truncated?: boolean }[]>(
      [{ ...row, output_truncated: "false" }],
      TaskMessageListSchema,
      [],
      { endpoint: "GET /api/tasks/:id/messages" },
    );
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.output).toBe("log line");
    expect(parsed[0]?.output_truncated).toBeUndefined();
  });

  it("keeps the surrounding rows when one row's flag is malformed", () => {
    const parsed = TaskMessageListSchema.parse([
      { ...row, seq: 1, output_truncated: true },
      { ...row, seq: 2, output_truncated: 12345 },
      { ...row, seq: 3, output_truncated: false },
    ]);
    expect(parsed.map((m) => m.seq)).toEqual([1, 2, 3]);
    expect(parsed.map((m) => m.output_truncated)).toEqual([true, undefined, false]);
  });

  it("falls back to an empty transcript when the response is not a list", () => {
    const parsed = parseWithFallback(
      { messages: "nope" },
      TaskMessageListSchema,
      [],
      { endpoint: "GET /api/tasks/:id/messages" },
    );
    expect(parsed).toEqual([]);
  });

  it("downgrades an unknown message type instead of dropping the transcript", () => {
    const parsed = TaskMessageListSchema.parse([{ ...row, type: "video" }]);
    expect(parsed[0]?.type).toBe("text");
  });
});


describe("AgentActivityBucketListSchema duration", () => {
  const bucket = { agent_id: "a", bucket_at: "2026-09-24T00:00:00Z", task_count: 201,
    completed_count: 201, failed_count: 0, cancelled_count: 0 };
  it("accepts optional aggregate duration from new and old servers", () => {
    expect(AgentActivityBucketListSchema.parse([bucket])[0]?.duration_ms).toBeUndefined();
    expect(AgentActivityBucketListSchema.parse([{ ...bucket, duration_ms: 12600000, duration_count: 201 }])[0]?.duration_count).toBe(201);
  });
  it("does not discard activity counts when duration is malformed", () => {
    const parsed = AgentActivityBucketListSchema.parse([{ ...bucket, duration_ms: "slow", duration_count: -1 }]);
    expect(parsed[0]?.task_count).toBe(201);
    expect(parsed[0]?.duration_ms).toBeUndefined();
    expect(parsed[0]?.duration_count).toBeUndefined();
  });
});

describe("QuestionSchema", () => {
  const wireQuestion = {
    id: "question-1",
    workspace_id: "ws-1",
    asker_agent_id: "agent-pieter",
    asked_in_task_id: "task-3",
    prompt: "Do invoices round per line or per total?",
    context_line: "The old system rounded per line.",
    answer_type: "single_choice",
    settings: { options: ["per line", "per total"], allow_other: true },
    suggested_answer: { option: "per line" },
    status: "answered",
    due_date: "2026-10-14",
    blocks_work: true,
    links: [
      { link_type: "issue", link_id: "issue-412", position: 0 },
      { link_type: "task", link_id: "task-3", position: 1 },
    ],
    answerers: [{ user_id: "user-brian", assigned_at: "2026-10-12T20:00:00Z" }],
    answers: [
      {
        id: "answer-1",
        answerer_user_id: "user-brian",
        value: { option: "per line" },
        display_text: "per line",
        note: "as the old system did",
        created_at: "2026-10-12T21:04:00Z",
        updated_at: "2026-10-12T21:04:00Z",
      },
    ],
    events: [
      { id: "event-1", kind: "asked", actor_type: "agent", actor_id: "agent-pieter", details: {}, created_at: "2026-10-12T20:00:00Z" },
      { id: "event-2", kind: "answered", actor_type: "member", actor_id: "user-brian", details: {}, created_at: "2026-10-12T21:04:00Z" },
    ],
    unseen_answer_count: 1,
    wake_available: true,
    created_at: "2026-10-12T20:00:00Z",
    updated_at: "2026-10-12T21:04:00Z",
  };

  const parse = (overrides: Record<string, unknown>) =>
    parseWithFallback<Question>({ ...wireQuestion, ...overrides }, QuestionSchema, EMPTY_QUESTION, {
      endpoint: "GET /api/questions/:id",
    });

  it("types settings, suggestion and answers by the answer type", () => {
    const question = parse({});
    expect(question.answer_type).toBe("single_choice");
    if (question.answer_type !== "single_choice") throw new Error("not narrowed");
    expect(question.settings).toEqual({ options: ["per line", "per total"], allow_other: true });
    expect(question.suggested_answer).toEqual({ option: "per line" });
    expect(question.answers[0]).toMatchObject({
      value: { option: "per line" },
      display_text: "per line",
      note: "as the old system did",
    });
    expect(question.links.map((link) => link.link_type)).toEqual(["issue", "task"]);
    expect(question.events.map((event) => event.kind)).toEqual(["asked", "answered"]);
    expect(question.unseen_answer_count).toBe(1);
    expect(question.wake_available).toBe(true);
  });

  it.each(["single_choice", "future_answer_type"])("preserves answer marks even for %s answers", (answer_type) => {
    const mark = { task_id: "run-1", agent_id: "thomas", seen_at: "2026-10-15T10:01:00Z", handled_at: "2026-10-15T10:02:00Z" };
    const question = parse({
      answer_type,
      answers: [{ ...wireQuestion.answers[0], run_marks: [mark, null, { task_id: "missing-agent" }] }],
    });
    expect(question.id).toBe(wireQuestion.id);
    expect(question.answers[0]?.run_marks).toEqual([mark]);
  });

  it.each([undefined, null, "not-a-list"])("keeps an answer with no readable marks list: %j", (run_marks) => {
    const question = parse({ answers: [{ ...wireQuestion.answers[0], run_marks }] });
    expect(question.answers[0]?.id).toBe("answer-1");
    expect(question.answers[0]?.run_marks).toBeUndefined();
  });

  it("reads a question of a set with its ask, policy and partly answered status", () => {
    const question = parse({
      ask_id: "question-0",
      ask_position: 2,
      ask_title: "Invoice rules",
      answer_policy: "each",
      status: "partly_answered",
    });
    expect(question).toMatchObject({
      id: "question-1",
      answer_type: "single_choice",
      ask_id: "question-0",
      ask_position: 2,
      ask_title: "Invoice rules",
      answer_policy: "each",
      status: "partly_answered",
    });
  });

  it("reads a question without ask fields as its own one-question ask", () => {
    expect(parse({})).toMatchObject({
      id: "question-1",
      ask_id: "question-1",
      ask_position: 0,
      ask_title: null,
      answer_policy: "any_one",
    });
  });

  it("reads a member's question asked outside a run, and an agent's question from before members asked", () => {
    expect(parse({ asker_agent_id: "", asked_in_task_id: "", asker_user_id: "user-skuddy" })).toMatchObject({
      id: "question-1",
      asker_agent_id: "",
      asked_in_task_id: "",
      asker_user_id: "user-skuddy",
    });
    expect(parse({})).toMatchObject({ id: "question-1", asker_agent_id: "agent-pieter", asker_user_id: "" });
  });

  it.each([["first_two"], [7], [null]])("reads an unknown or malformed answer policy %j as any_one", (policy) => {
    const question = parse({ ask_id: "question-0", answer_policy: policy });
    expect(question).toMatchObject({ id: "question-1", ask_id: "question-0", answer_policy: "any_one" });
  });

  it("degrades malformed ask fields instead of failing the question", () => {
    expect(parse({ ask_id: 42, ask_position: -1, ask_title: 7 })).toMatchObject({
      id: "question-1",
      ask_id: "question-1",
      ask_position: 0,
      ask_title: null,
    });
    expect(parse({ ask_id: "", ask_position: 1.5 })).toMatchObject({ ask_id: "question-1", ask_position: 0 });
    expect(parse({ ask_id: null, ask_position: "2" })).toMatchObject({ ask_id: "question-1", ask_position: 0 });
  });

  it("keeps the ask of a question it reads as unknown", () => {
    const question = parse({
      answer_type: "color_picker",
      ask_id: "question-0",
      ask_position: 1,
      ask_title: "Brand",
      answer_policy: "each",
    });
    expect(question).toMatchObject({
      answer_type: "unknown",
      ask_id: "question-0",
      ask_position: 1,
      ask_title: "Brand",
      answer_policy: "each",
    });
  });

  it.each([
    ["text", { length: "long", placeholder: "Explain" }, { text: "Rounded per line", attachment_ids: ["att-1"] }],
    ["yes_no", { why_on_no: true }, { choice: "no" }],
    ["single_choice", { options: ["a", "b"], allow_other: true }, { other: "per invoice" }],
    ["multiple_choice", { options: ["a", "b", "c"], min: 2, max: 3 }, { options: ["a", "c"] }],
    ["confirm_facts", { facts: ["VAT 21%", "EUR"] }, { facts: [{ correct: true }, { correct: false, correction: "USD" }] }],
    ["ranking", { items: ["a", "b"] }, { order: ["b", "a"] }],
    ["scale", { min: 1, max: 5, min_label: "Low", max_label: "High" }, { point: 4 }],
    ["number", { unit: "EUR", min: 0, max: 100 }, { number: 42.5 }],
    ["date", { kind: "range" }, { start: "2026-10-12", end: "2026-10-14" }],
    ["person", { allow: "both" }, { type: "agent", id: "agent-joeri" }],
    ["file", { accept: ["application/pdf", ".csv"] }, { attachment_ids: ["att-1"] }],
    ["approval", { subject: "Release 1.2", link: { title: "PR", url: "https://example.test/pr/1" } }, { decision: "request_changes" }],
  ])("reads a %s question with its settings and value", (answerType, settings, value) => {
    const question = parse({
      answer_type: answerType,
      settings,
      suggested_answer: null,
      answers: [{ ...wireQuestion.answers[0], value }],
    });
    expect(question.answer_type).toBe(answerType);
    expect(question.settings).toEqual(settings);
    expect(question.answers[0]?.value).toEqual(value);
  });

  it.each([
    ["text", { length: "short" }],
    ["ranking", { items: ["a", "b"] }],
  ])("reads a %s question with a declined answer by its answer type", (answerType, settings) => {
    const question = parse({
      answer_type: answerType,
      settings,
      suggested_answer: null,
      answers: [{ ...wireQuestion.answers[0], value: { declined: true }, display_text: "Declined to answer", note: "Not my area" }],
    });
    expect(question.answer_type).toBe(answerType);
    expect(question.answers[0]).toMatchObject({ value: { declined: true }, display_text: "Declined to answer", note: "Not my area" });
  });

  it("defaults missing optional fields and arrays", () => {
    const {
      context_line: _contextLine,
      suggested_answer: _suggestion,
      due_date: _dueDate,
      blocks_work: _blocksWork,
      links: _links,
      answerers: _answerers,
      answers: _answers,
      events: _events,
      unseen_answer_count: _unseen,
      wake_available: _wake,
      ...core
    } = wireQuestion;
    const question = parseWithFallback<Question>(
      { ...core, status: "open", settings: { options: ["a", "b"] } },
      QuestionSchema,
      EMPTY_QUESTION,
      { endpoint: "GET /api/questions/:id" },
    );
    expect(question).toMatchObject({
      id: "question-1",
      answer_type: "single_choice",
      settings: { options: ["a", "b"], allow_other: false },
      suggested_answer: null,
      context_line: null,
      due_date: null,
      blocks_work: false,
      links: [],
      answerers: [],
      answers: [],
      events: [],
      unseen_answer_count: 0,
      wake_available: false,
    });
  });

  it("reads an unknown answer type as unknown, keeping the prompt and display text", () => {
    const question = parse({ answer_type: "color_picker", settings: { palette: "warm" } });
    expect(question.id).toBe("question-1");
    expect(question.prompt).toBe(wireQuestion.prompt);
    expect(question.answer_type).toBe("unknown");
    expect(question.settings).toEqual({ palette: "warm" });
    expect(question.suggested_answer).toBeNull();
    expect(question.answers[0]?.display_text).toBe("per line");
  });

  it.each([
    ["settings", { settings: { options: "per line" } }],
    ["a suggested answer", { suggested_answer: { option: 3 } }],
    ["an answer value", { answers: [{ ...wireQuestion.answers[0], value: ["per line"] }] }],
    ["a missing answer value", { answers: [{ ...wireQuestion.answers[0], value: undefined }] }],
  ])("reads a question with a wrongly shaped %s as unknown", (_part, overrides) => {
    const question = parse(overrides);
    expect(question.id).toBe("question-1");
    expect(question.answer_type).toBe("unknown");
    expect(question.answers.map((answer) => answer.display_text)).toEqual(["per line"]);
  });

  it("reads an unknown or missing status as closed", () => {
    expect(parse({ status: "expired" }).status).toBe("closed");
    expect(parse({ status: undefined }).status).toBe("closed");
  });

  it("reads a cancelled question with its cancel event and every answer sent before it", () => {
    const question = parse({
      status: "cancelled",
      events: [
        { id: "event-1", kind: "cancelled", actor_type: "member", actor_id: "user-brian", details: { note: "Not needed any more" }, created_at: "2026-10-12T22:00:00Z" },
        { id: "event-2", kind: "cancelled", actor_type: "member", actor_id: "user-brian", details: {}, created_at: "2026-10-12T22:01:00Z" },
        { id: "event-3", kind: "cancelled", actor_type: "member", actor_id: "user-brian", details: { note: "" }, created_at: "2026-10-12T22:02:00Z" },
        { id: "event-4", kind: "cancelled", actor_type: "member", actor_id: "user-brian", details: { note: 7 }, created_at: "2026-10-12T22:03:00Z" },
      ],
    });
    expect(question.status).toBe("cancelled");
    expect(question.answers.map((answer) => answer.display_text)).toEqual(["per line"]);
    expect(question.events.map((event) => [event.kind, event.details])).toEqual([
      ["cancelled", { note: "Not needed any more" }],
      ["cancelled", { note: null }],
      ["cancelled", { note: null }],
      ["cancelled", { note: null }],
    ]);
  });

  it("reads a chat link with its member and a chat wake target", () => {
    const question = parse({
      links: [
        { link_type: "chat", link_id: "chat-1", position: 0, member_id: "user-brian" },
        { link_type: "task", link_id: "task-3", position: 1 },
      ],
      wake_targets: [
        { agent_id: "agent-pieter", issue_id: "", chat_session_id: "chat-1", unseen_answer_count: 2, wake_available: true },
        { agent_id: "agent-joeri", issue_id: "issue-412", unseen_answer_count: 1, wake_available: true },
      ],
    });
    expect(question.links).toEqual([
      { link_type: "chat", link_id: "chat-1", position: 0, member_id: "user-brian" },
      { link_type: "task", link_id: "task-3", position: 1, member_id: null },
    ]);
    expect(question.wake_targets).toEqual([
      { agent_id: "agent-pieter", issue_id: null, chat_session_id: "chat-1", unseen_answer_count: 2, wake_available: true },
      { agent_id: "agent-joeri", issue_id: "issue-412", chat_session_id: null, unseen_answer_count: 1, wake_available: true },
    ]);
  });

  it("drops a link of an unknown type and a wake target without an issue or a chat", () => {
    const question = parse({
      links: [
        { link_type: "whiteboard", link_id: "board-1", position: 0 },
        { link_type: "issue", link_id: "issue-412", position: 1 },
      ],
      wake_targets: [
        { agent_id: "agent-pieter", issue_id: "", chat_session_id: null, unseen_answer_count: 1, wake_available: true },
        { agent_id: "agent-joeri", issue_id: "issue-412", unseen_answer_count: 1, wake_available: true },
      ],
    });
    expect(question.id).toBe("question-1");
    expect(question.links).toEqual([{ link_type: "issue", link_id: "issue-412", position: 1, member_id: null }]);
    expect(question.wake_targets.map((target) => target.agent_id)).toEqual(["agent-joeri"]);
  });

  it("drops events and links it cannot read and keeps the rest", () => {
    const question = parse({
      links: [
        { link_type: "issue", link_id: "issue-412", position: 0 },
        { link_type: "project", link_id: "project-1", position: 1 },
        { link_type: "future_link_type", link_id: "future-1", position: 2 },
        { link_type: "task", link_id: 7, position: 3 },
      ],
      events: [
        { id: "event-1", kind: "pinned", actor_type: "member", actor_id: "user-brian", details: {}, created_at: "2026-10-12T20:00:00Z" },
        { id: "event-2", kind: "reassigned", actor_type: "member", actor_id: "user-brian", details: { note: "no user" }, created_at: "2026-10-12T20:01:00Z" },
        { id: "event-3", kind: "ask_back", actor_type: "member", actor_id: "user-brian", details: { text: "Which customer?" }, created_at: "2026-10-12T20:02:00Z" },
        { id: "event-4", kind: "reassigned", actor_type: "member", actor_id: "user-brian", details: { user_id: "user-anouk", note: "You set the invoice rules" }, created_at: "2026-10-12T20:03:00Z" },
        { id: "event-5", kind: "woken", actor_type: "member", actor_id: "user-brian", details: { task_id: "task-4", answer_count: 1 }, created_at: "2026-10-12T21:05:00Z" },
        { id: "event-6", kind: "closed", actor_type: "agent", actor_id: null, created_at: "2026-10-12T22:00:00Z" },
      ],
    });
    expect(question.links).toEqual([
      { link_type: "issue", link_id: "issue-412", position: 0, member_id: null },
      { link_type: "project", link_id: "project-1", position: 1, member_id: null },
    ]);
    expect(question.events.map((event) => [event.kind, event.details])).toEqual([
      ["ask_back", { text: "Which customer?" }],
      ["reassigned", { user_id: "user-anouk", note: "You set the invoice rules" }],
      ["woken", { task_id: "task-4", answer_count: 1 }],
      ["closed", {}],
    ]);
  });

  it("reads non-list links and events as empty instead of dropping the question", () => {
    const question = parse({ links: "issue-412", events: { kind: "asked" } });
    expect(question.id).toBe("question-1");
    expect(question.links).toEqual([]);
    expect(question.events).toEqual([]);
  });

  it("degrades wrongly typed defaults instead of failing the question", () => {
    const question = parse({ blocks_work: "yes", wake_available: 1, unseen_answer_count: -2, due_date: 20261014 });
    expect(question).toMatchObject({
      id: "question-1",
      blocks_work: false,
      wake_available: false,
      unseen_answer_count: 0,
      due_date: null,
    });
  });

  it("falls back to the empty question when its core is malformed", () => {
    expect(parse({ id: 42 })).toEqual(EMPTY_QUESTION);
    expect(parse({ answers: "per line" })).toEqual(EMPTY_QUESTION);
    expect(parse({ answerers: [{ assigned_at: "2026-10-12T20:00:00Z" }] })).toEqual(EMPTY_QUESTION);
    expect(
      parseWithFallback<Question>("not a question", QuestionSchema, EMPTY_QUESTION, {
        endpoint: "GET /api/questions/:id",
      }),
    ).toEqual(EMPTY_QUESTION);
  });
});

describe("QuestionListSchema", () => {
  const question = {
    id: "question-1",
    workspace_id: "ws-1",
    asker_agent_id: "agent-pieter",
    asked_in_task_id: "task-3",
    prompt: "Ship it?",
    answer_type: "yes_no",
    settings: { why_on_no: false },
    status: "open",
    created_at: "2026-10-12T20:00:00Z",
    updated_at: "2026-10-12T20:00:00Z",
  };
  const parseList = (data: unknown) =>
    parseWithFallback<Question[]>(data, QuestionListSchema, EMPTY_QUESTION_LIST, {
      endpoint: "GET /api/issues/:id/questions",
    });

  it("drops a malformed question without hiding the others", () => {
    const questions = parseList([
      question,
      { ...question, id: "question-2", prompt: null },
      { ...question, id: "question-3", answer_type: "sketch" },
    ]);
    expect(questions.map((q) => [q.id, q.answer_type])).toEqual([
      ["question-1", "yes_no"],
      ["question-3", "unknown"],
    ]);
  });

  it("falls back to an empty list when the response is not a list", () => {
    expect(parseList({ questions: [question] })).toEqual(EMPTY_QUESTION_LIST);
    expect(parseList(undefined)).toEqual([]);
  });
});

describe("QuestionWakeResultSchema", () => {
  const parseWake = (data: unknown) =>
    parseWithFallback<QuestionWakeResult>(data, QuestionWakeResultSchema, EMPTY_QUESTION_WAKE_RESULT, {
      endpoint: "POST /api/questions/:id/wake",
    });

  it.each([
    { task_id: "task-4", answer_count: 1 },
    { task_id: "", answer_count: 2 },
    { task_id: "", answer_count: 0 },
  ])("reads an accepted wake response without fallback: %j", (response) => {
    const result = parseWake(response);
    expect(result).toEqual(response);
    expect(result).not.toBe(EMPTY_QUESTION_WAKE_RESULT);
  });

  it("defaults a missing or malformed answer count", () => {
    expect(parseWake({ task_id: "task-4" })).toEqual({ task_id: "task-4", answer_count: 0 });
    expect(parseWake({ task_id: "task-4", answer_count: "one" })).toEqual({ task_id: "task-4", answer_count: 0 });
  });

  it.each([{ answer_count: 1 }, { task_id: null, answer_count: 1 }, { task_id: 42, answer_count: 1 }, null])(
    "falls back when the task id is missing or malformed: %j",
    (response) => {
      expect(parseWake(response)).toBe(EMPTY_QUESTION_WAKE_RESULT);
    },
  );
});

describe("QuestionRowPageSchema", () => {
  const wireRow = {
    id: "question-1",
    ask_id: "question-1",
    ask_position: 0,
    ask_title: null,
    prompt: "Ship it?",
    answer_type: "yes_no",
    settings: { why_on_no: false },
    status: "open",
    answer_policy: "any_one",
    asker_agent_id: "agent-pieter",
    answerer_ids: ["user-brian"],
    due_date: null,
    blocks_work: false,
    created_at: "2026-10-12T20:00:00Z",
    link: { type: "issue", id: "issue-412", identifier: "WORK-412", title: "Invoices", status: "todo" },
    via: null,
    open_issue: { id: "issue-412", identifier: "WORK-412" },
    my_answer: null,
  };
  const parsePage = (rows: unknown[]) =>
    parseWithFallback<QuestionRowPage>({ rows, next_cursor: null }, QuestionRowPageSchema, EMPTY_QUESTION_ROW_PAGE, {
      endpoint: "GET /api/questions",
    }).rows;

  it("reads a cancelled row", () => {
    expect(parsePage([{ ...wireRow, status: "cancelled" }])[0]?.status).toBe("cancelled");
  });

  it("reads a chat row by its member, with no issue to open", () => {
    const [row] = parsePage([{
      ...wireRow,
      link: { type: "chat", id: "chat-1", title: "", member_id: "user-brian" },
      open_issue: { id: "", identifier: "" },
    }]);
    expect(row).toMatchObject({
      id: "question-1",
      link: { type: "chat", id: "chat-1", identifier: null, title: "", status: null, member_id: "user-brian" },
      open_issue: null,
    });
  });

  it("reads an unknown row link type as an issue, and an issue row without open_issue by its link", () => {
    const [row] = parsePage([{ ...wireRow, link: { ...wireRow.link, type: "whiteboard" }, open_issue: undefined }]);
    expect(row).toMatchObject({
      link: { type: "issue", id: "issue-412", identifier: "WORK-412", member_id: null },
      open_issue: { id: "issue-412", identifier: "WORK-412" },
    });
  });
});

describe("RepositoryCheckoutRequestSchema", () => {
  const parseCheck = (data: unknown) =>
    parseWithFallback<RepositoryCheckoutRequest>(
      data,
      RepositoryCheckoutRequestSchema,
      MALFORMED_REPOSITORY_CHECKOUT_REQUEST,
      { endpoint: "GET /api/runtimes/{id}/repository-checkouts/{requestId}" },
    );

  it("reads the daemon's answer the server nests under result", () => {
    const check = parseCheck({
      id: "check-1",
      runtime_id: "runtime-1",
      status: "completed",
      result: {
        status: "ok",
        resolved_path: "/Users/brian/Repos/rebl",
        passed: true,
        failures: null,
        version_files: ["package.json"],
        last_tag: "v0.4.2",
        install_commands: [{ file: "pnpm-lock.yaml", command: "pnpm install" }],
      },
    });
    expect(check).toMatchObject({
      status: "completed",
      resolved_path: "/Users/brian/Repos/rebl",
      passed: true,
      failures: [],
      version_files: ["package.json"],
      last_tag: "0.4.2",
      install_commands: [{ file: "pnpm-lock.yaml", command: "pnpm install" }],
    });
  });

  it("reads a failed check's reasons from its result", () => {
    const check = parseCheck({
      id: "check-1",
      runtime_id: "runtime-1",
      status: "completed",
      result: {
        passed: false,
        failures: [{ code: "origin_mismatch", detail: "git@github.com:appboypov/rebl.git" }],
      },
    });
    expect(check.passed).toBe(false);
    expect(check.failures).toEqual([
      { code: "origin_mismatch", detail: "git@github.com:appboypov/rebl.git" },
    ]);
  });

  it("keeps a pending check pending before the daemon answers", () => {
    expect(parseCheck({ id: "check-1", runtime_id: "runtime-1", status: "pending" })).toMatchObject({
      status: "pending",
      passed: false,
      failures: [],
    });
  });
});

describe("AgentSessionsResponseSchema", () => {
  const parseSessions = (data: unknown) =>
    parseWithFallback<AgentSessionsResponse>(
      data,
      AgentSessionsResponseSchema,
      EMPTY_AGENT_SESSIONS_RESPONSE,
      { endpoint: "GET /api/agents/{id}/sessions" },
    );

  const terminal = {
    id: "session-1",
    kind: "terminal",
    pid: 4242,
    herdr_pane_id: "pane-7",
    runtime: null,
    task_id: null,
    issue: null,
    work_dir: "/Users/brian/Worktrees/das/multica/work-1057",
    repository: "appboypov/multica",
    branch: "work-1057",
    status: "working",
    started_at: "2026-10-15T16:44:00Z",
  };

  const run = {
    id: "task-1",
    kind: "run",
    pid: null,
    herdr_pane_id: null,
    runtime: { id: "runtime-1", name: "Brian's Mac" },
    task_id: "task-1",
    issue: { id: "issue-1", identifier: "WORK-1057" },
    work_dir: "/Users/brian/multica_workspaces/work-1057/workdir",
    repository: null,
    branch: null,
    status: "starting",
    started_at: "2026-10-15T16:45:00Z",
  };

  it("parses a terminal session and a run", () => {
    expect(parseSessions({ sessions: [terminal, run] })).toEqual({
      sessions: [terminal, run],
    });
  });

  it("reads a response without sessions as none", () => {
    expect(parseSessions({})).toEqual({ sessions: [] });
    expect(parseSessions({ sessions: "nope" })).toEqual({ sessions: [] });
  });

  it("falls back to no sessions on a non-object body", () => {
    expect(parseSessions(null)).toEqual(EMPTY_AGENT_SESSIONS_RESPONSE);
  });

  it("keeps a session of an unknown kind as unknown", () => {
    const parsed = parseSessions({ sessions: [{ ...terminal, kind: "cloud" }] });
    expect(parsed.sessions).toHaveLength(1);
    expect(parsed.sessions[0]).toMatchObject({ id: "session-1", kind: "unknown", status: "working" });
  });

  it("keeps a session of an unknown status as unknown", () => {
    const parsed = parseSessions({ sessions: [{ ...terminal, status: "sleeping" }] });
    expect(parsed.sessions).toHaveLength(1);
    expect(parsed.sessions[0]).toMatchObject({ id: "session-1", kind: "terminal", status: "unknown" });
  });

  it("keeps a run without a runtime, with the runtime null", () => {
    const { runtime: _runtime, ...withoutRuntime } = run;
    const parsed = parseSessions({ sessions: [withoutRuntime, { ...run, id: "task-2", runtime: "rt" }] });
    expect(parsed.sessions).toHaveLength(2);
    expect(parsed.sessions[0]).toMatchObject({
      id: "task-1",
      kind: "run",
      runtime: null,
      issue: { id: "issue-1", identifier: "WORK-1057" },
    });
    expect(parsed.sessions[1]?.runtime).toBeNull();
  });

  it("defaults missing strings to empty and missing nullable values to null", () => {
    const parsed = parseSessions({ sessions: [{ id: "session-1" }] });
    expect(parsed.sessions).toEqual([
      {
        id: "session-1",
        kind: "unknown",
        pid: null,
        herdr_pane_id: null,
        runtime: null,
        task_id: null,
        issue: null,
        work_dir: "",
        repository: null,
        branch: null,
        status: "unknown",
        started_at: "",
      },
    ]);
  });
});

describe("release owner and release page responses", () => {
  const release = {
    ...EMPTY_RELEASE,
    id: "release-1",
    repository_id: "repo-1",
    version: "1.1.0",
    title: "Account and profile",
  };
  const parseDetail = (body: unknown) =>
    parseWithFallback(body, ReleaseDetailResponseSchema, EMPTY_RELEASE_DETAIL_RESPONSE, {
      endpoint: "/api/releases/release-1",
    }).release;

  it("keeps a known owner with its name and avatar", () => {
    expect(parseDetail({
      release: { ...release, owner_type: "agent", owner_id: "agent-1", owner_name: "Kees", owner_avatar_url: "/a.png" },
    })).toMatchObject({ id: "release-1", owner_type: "agent", owner_id: "agent-1", owner_name: "Kees", owner_avatar_url: "/a.png" });
  });

  it("reads a missing, unknown or half-sent owner as no owner and keeps the release", () => {
    const noOwner = { owner_type: null, owner_id: null, owner_name: null, owner_avatar_url: null };
    const { owner_type: _type, owner_id: _id, owner_name: _name, owner_avatar_url: _avatar, ...older } = release;
    for (const owner of [
      {},
      { owner_type: "member", owner_id: "user-1", owner_name: "Brian" },
      { owner_type: "agent", owner_id: null, owner_name: "Kees" },
      { owner_type: null, owner_id: "agent-1" },
      { owner_type: 7, owner_id: ["agent-1"] },
    ]) {
      expect(parseDetail({ release: { ...older, ...owner } })).toMatchObject({ id: "release-1", title: "Account and profile", ...noOwner });
    }
    const listed = parseWithFallback(
      { releases: [{ ...older, owner_type: "robot", owner_id: "x" }] },
      ReleasesResponseSchema,
      { releases: [] },
      { endpoint: "/api/releases" },
    );
    expect(listed.releases).toEqual([expect.objectContaining({ id: "release-1", ...noOwner })]);
  });

  it("drops only malformed release issues and comments, keeping order", () => {
    const issue = { id: "i-1", identifier: "WORK-501", title: "Sign up", status: "todo", assignee_type: "agent", assignee_id: "a-1", parent_issue_id: null, worklane_id: "w-1", worklane_name: "Fan accounts", pull_requests: [] };
    expect(ReleaseIssuesResponseSchema.parse({ issues: [issue, { title: "no id" }, { ...issue, id: "i-2", identifier: "WORK-502" }] }).issues.map((row) => row.identifier))
      .toEqual(["WORK-501", "WORK-502"]);
    const comment = { id: "c-1", release_id: "release-1", author_type: "member", author_id: "u-1", content: "Start", parent_id: null, created_at: "2026-01-01T00:00:00Z" };
    const comments = ReleaseCommentsSchema.parse([comment, { ...comment, id: "c-2", author_type: "robot" }, { ...comment, id: "c-3", parent_id: "c-1" }, { ...comment, id: "" }]);
    expect(comments.map((row) => [row.id, row.author_type])).toEqual([["c-1", "member"], ["c-2", "system"], ["c-3", "member"]]);
  });

  it("keeps only owner changes in release activity", () => {
    const entries = parseWithFallback({ entries: [
      { id: "e-1", actor_type: "member", actor_id: "u-1", created_at: "", action: "release_planned", details: {} },
      {
        id: "e-2", actor_type: "member", actor_id: "u-1", created_at: "", action: "release_owner_changed",
        details: { release_id: "release-1", version: "1.1.0", title: "Account and profile", owner_type: "agent", owner_id: "a-1", owner_name: "Pieter", previous_owner_type: null, previous_owner_id: null, previous_owner_name: null },
      },
    ] }, ReleaseActivityResponseSchema, EMPTY_RELEASE_ACTIVITY_RESPONSE, { endpoint: "/api/releases/release-1/activity" }).entries;
    expect(entries.map((entry) => entry.id)).toEqual(["e-2"]);
  });
});
