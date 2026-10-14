import { z } from "zod";
import { normalizeIssueStatusCategory } from "../issues/config/status";
import { ISSUE_TYPES } from "../types/issue";
import type {
  AgentBuilderRuntimeSwitch,
  AgentBuilderSession,
  AgentBuilderSessionSummary,
  Attachment,
  AutopilotRun,
  BillingBalance,
  BillingBatchesPage,
  BillingCheckoutSessionStatus,
  BillingPriceTier,
  BillingTopupsPage,
  BillingTransactionsPage,
  CancelTaskResponse,
  ChatMessage,
  ChatDraftRestoresResponse,
  ChatPendingTask,
  ChatSession,
  PrioritizeQueuedChatTaskResponse,
  SendChatMessageResponse,
  StartMikaOnboardingResponse,
  Comment,
  CreateBillingCheckoutSessionResponse,
  CreateBillingPortalSessionResponse,
  WorkspaceSubscriptionEntitlements,
  WorkspaceSubscriptionSummary,
  IssueLimitUsage,
  WorkspaceSubscriptionPrice,
  WorkspaceSubscriptionPrices,
  CreateWorkspaceSubscriptionCheckoutResponse,
  WorkspaceSubscriptionSeatReconcileResult,
  WorkspaceSeatPurchasePreview,
  PurchaseWorkspaceSeatsResponse,
  CreateWorkspaceSubscriptionPortalResponse,
  CronPreviewResponse,
  DingTalkInstallation,
  ListDingTalkInstallationsResponse,
  ListDingTalkGroupsResponse,
  RedeemDingTalkBindingTokenResponse,
  WecomInstallation,
  ListWecomInstallationsResponse,
  RedeemWecomBindingTokenResponse,
  TelegramInstallation,
  ListTelegramInstallationsResponse,
  RedeemTelegramBindingTokenResponse,
  GroupedIssuesResponse,
  GitHubConnectResponse,
  IssuePullRequestsResponse,
  InboxItem,
  InboxWorkspaceUnread,
  AgentQuestionsSettings,
  AnswerType,
  Question,
  QuestionOpenCount,
  QuestionRow,
  QuestionRowPage,
  QuestionWakeResult,
  UnreadableQuestion,
  UnreadableQuestionRow,
  Label,
  MemberWithUser,
  IssueProperty,
  ListPropertiesResponse,
  QuickAction,
  ListQuickActionsResponse,
  IssuePropertiesResponse,
  IssueTableGroupDescriptor,
  IssueTableFacetsResponse,
  IssueTableGroupsResponse,
  IssueTableRowsResponse,
  ListIssuesResponse,
  ListGitHubInstallationsResponse,
  ListGitHubRepositoriesResponse,
  ListLabelsResponse,
  ListAvailableRepositoriesResponse,
  ListOrganisationsResponse,
  ListProjectsResponse,
  ListRepositoriesResponse,
  ListWorklanesResponse,
  ListWebhookDeliveriesResponse,
  IssueStatusEntry,
  ListIssueStatusesResponse,
  NotificationPreferenceResponse,
  Organisation,
  PluginInstallation,
  PluginInstallationListResponse,
  PluginPackage,
  PluginPackageListResponse,
  PluginPreview,
  PluginSurfaceLaunch,
  Project,
  AvailableRepository,
  Repository,
  RepositoryCheckoutRequest,
  ResourceLabelsResponse,
  RuntimeModelListRequest,
  RuntimeDirectoryListingRequest,
  SearchIssuesResponse,
  SearchProjectsResponse,
  ShareLink,
  ShareLinkInfo,
  Skill,
  SkillImportResult,
  SkillSummary,
  Squad,
  TimelineEntry,
  User,
  WebhookDelivery,
  WorkspaceMcpServer,
  VersionPolicy,
  Worklane,
  WorklaneActivityEntry,
  WorklaneActivityResponse,
  WorklaneRequest,
  WorklaneRequestReasonCode,
  Release,
  ReleaseLineResponse,
  ListReleasesResponse,
  ReleaseRefusal,
  RepositoryRefusal,
  IssueTemplate,
  IssueTemplatesResponse,
  WorklaneIssuesResponse,
  ReleaseProgressResponse,
  IssuePlacementRefusal,
} from "../types";
import type { CloudRuntimeNode } from "../runtimes/cloud-runtime";
import type { CreateFeedbackResponse } from "../feedback/types";

export const PluginConfigFieldSchema = z.object({
  key: z.string(),
  type: z.string().default("string"),
  label: z.string().default(""),
  description: z.string().optional(),
  required: z.boolean().default(false),
  options: z.array(z.string()).default([]),
  placeholder: z.string().optional(),
  multiline: z.boolean().default(false),
}).loose();

export const PluginSurfaceSchema = z.object({
  key: z.string(),
  type: z.string().default(""),
  name: z.string().default(""),
  entry: z.string().default(""),
  platforms: z.array(z.string()).default([]),
}).loose();

export const PluginHookSchema = z.object({
  key: z.string(),
  name: z.string().default(""),
  description: z.string().default(""),
  triggers: z.array(z.string()).default([]),
  events: z.array(z.string()).default([]),
  schedule: z.object({
    cron: z.string().default(""),
    timezone: z.string().default(""),
    next_run_at: z.string().optional(),
  }).loose().optional(),
  transport: z.string().default(""),
}).loose();

export const PluginResourceSchema = z.object({
  type: z.string().default(""),
  key: z.string(),
  entry: z.string().default(""),
}).loose();

export const PluginInstallationSchema = z.object({
  id: z.string(),
  plugin_key: z.string().default(""),
  name: z.string().default(""),
  description: z.string().optional(),
  version: z.string().default(""),
  package_version_id: z.string().default(""),
  enabled: z.boolean().default(false),
  granted_scopes: z.array(z.string()).default([]),
  config_schema: z.array(PluginConfigFieldSchema).default([]),
  config: z.record(z.string(), z.unknown()).default({}),
  configured_secrets: z.array(z.string()).default([]),
  surfaces: z.array(PluginSurfaceSchema).default([]),
  hooks: z.array(PluginHookSchema).default([]),
  resources: z.array(PluginResourceSchema).default([]),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_PLUGIN_INSTALLATION: PluginInstallation = {
  id: "",
  plugin_key: "",
  name: "",
  version: "",
  package_version_id: "",
  enabled: false,
  granted_scopes: [],
  config_schema: [],
  config: {},
  configured_secrets: [],
  surfaces: [],
  hooks: [],
  resources: [],
  created_at: "",
  updated_at: "",
};

export const PluginInstallationListResponseSchema = z.object({
  plugins: z.array(PluginInstallationSchema).default([]),
}).loose();

export const EMPTY_PLUGIN_INSTALLATION_LIST: PluginInstallationListResponse = {
  plugins: [],
};

/**
 * One completed hook call. `status` is the host's classification, not the
 * endpoint's: "refused" means we declined to make the call at all, which is a
 * different problem for the reader than an endpoint that answered badly.
 */
export const PluginHookResultSchema = z.object({
  status: z.string().default("ok"),
  output: z.unknown().optional(),
  error: z.string().optional(),
  latency_ms: z.number().default(0),
  hook_key: z.string().default(""),
  trigger: z.string().default(""),
  attempts: z.number().default(1),
}).loose();

export const PluginInvocationSchema = z.object({
  id: z.string().default(""),
  hook_key: z.string().default(""),
  trigger: z.string().default(""),
  status: z.string().default(""),
  event_type: z.string().optional(),
  attempt: z.number().default(1),
  latency_ms: z.number().default(0),
  error: z.string().optional(),
  delivery_id: z.string().optional(),
  planned_at: z.string().optional(),
  created_at: z.string().default(""),
}).loose();

export const PluginInvocationListSchema = z.object({
  invocations: z.array(PluginInvocationSchema).default([]),
}).loose();

/**
 * Returned once, by the request that minted it. There is no read endpoint for
 * either value, so a client that discards this cannot recover it.
 */
export const PluginTokenIssueSchema = z.object({
  token: z.string().default(""),
  signing_secret: z.string().default(""),
}).loose();

/**
 * Discovered tools for one `mcp`-transport hook.
 *
 * Defaults matter here in the usual direction: an unparseable response yields
 * an EMPTY list and nothing approved, so a drifted backend cannot make the UI
 * render a tool as already-approved.
 */
export const PluginMCPToolSchema = z.object({
  name: z.string().default(""),
  description: z.string().default(""),
  schema_digest: z.string().default(""),
  approved: z.boolean().default(false),
  drifted: z.boolean().default(false),
}).loose();

export const PluginMCPToolListSchema = z.object({
  tools: z.array(PluginMCPToolSchema).default([]),
}).loose();

export const PluginManifestSummarySchema = z.object({
  key: z.string().default(""),
  name: z.string().default(""),
  description: z.string().optional(),
  version: z.string().default(""),
  author: z.object({
    name: z.string().default(""),
    url: z.string().optional(),
  }).loose().default({ name: "" }),
  contributes: z.object({
    hooks: z.array(z.object({
      key: z.string().default(""),
      name: z.string().default(""),
      triggers: z.array(z.string()).default([]),
      schedule: z.object({
        cron: z.string().default(""),
        timezone: z.string().default(""),
      }).loose().optional(),
    }).loose()).default([]),
  }).loose().optional(),
}).loose();

export const PluginPreviewSchema = z.object({
  manifest: PluginManifestSummarySchema,
  scopes: z.array(z.string()).default([]),
  config_schema: z.array(PluginConfigFieldSchema).default([]),
  version_id: z.string().default(""),
  version: z.string().default(""),
  digest: z.string().default(""),
  installed: z.boolean().default(false),
  installed_version: z.string().optional(),
  added_scopes: z.array(z.string()).default([]),
}).loose();

export const EMPTY_PLUGIN_PREVIEW: PluginPreview = {
  manifest: { key: "", name: "", version: "", author: { name: "" } },
  scopes: [],
  config_schema: [],
  version_id: "",
  version: "",
  digest: "",
  installed: false,
  added_scopes: [],
};

/**
 * A published version. `installed` is the marker the settings page reads to
 * answer "which one am I on"; it defaults to false so a malformed response can
 * never claim a version is running that is not.
 */
export const PluginPackageVersionSchema = z.object({
  id: z.string().default(""),
  version: z.string().default(""),
  digest: z.string().default(""),
  size_bytes: z.number().default(0),
  published_at: z.string().default(""),
  installed: z.boolean().default(false),
}).loose();

export const PluginPackageSchema = z.object({
  id: z.string().default(""),
  plugin_key: z.string().default(""),
  name: z.string().default(""),
  versions: z.array(PluginPackageVersionSchema).default([]),
  created_at: z.string().default(""),
}).loose();

export const PluginPackageListResponseSchema = z.object({
  packages: z.array(PluginPackageSchema).default([]),
}).loose();

export const EMPTY_PLUGIN_PACKAGE_LIST: PluginPackageListResponse = {
  packages: [],
};

export const EMPTY_PLUGIN_PACKAGE: PluginPackage = {
  id: "",
  plugin_key: "",
  name: "",
  versions: [],
  created_at: "",
};

/** A malformed launch becomes unavailable, never a partly trusted frame. */
export const PluginSurfaceLaunchSchema = z.object({
  url: z.string().default(""),
  bridge_token: z.string().default(""),
  version: z.string().default(""),
  digest: z.string().default(""),
}).loose();

export const EMPTY_PLUGIN_SURFACE_LAUNCH: PluginSurfaceLaunch = {
  url: "",
  bridge_token: "",
  version: "",
  digest: "",
};

export const GitHubInstallationSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  installation_id: z.number().optional(),
  account_login: z.string(),
  account_type: z.string(),
  account_avatar_url: z.string().nullable(),
  created_at: z.string(),
  connected_by: z.string().optional(),
}).loose();

export const ListGitHubInstallationsResponseSchema = z.object({
  installations: z.array(GitHubInstallationSchema).default([]),
  configured: z.boolean().optional().default(false),
  repository_browse_configured: z.boolean().optional().default(false),
  can_manage: z.boolean().optional().default(false),
}).loose();

export const EMPTY_LIST_GITHUB_INSTALLATIONS_RESPONSE: ListGitHubInstallationsResponse = {
  installations: [],
  configured: false,
  repository_browse_configured: false,
  can_manage: false,
};

export const GitHubConnectResponseSchema = z.object({
  url: z.string().optional(),
  configured: z.boolean().optional().default(false),
}).loose();

export const EMPTY_GITHUB_CONNECT_RESPONSE: GitHubConnectResponse = {
  configured: false,
};

export const GitHubRepositorySchema = z.object({
  id: z.number(),
  full_name: z.string(),
  html_url: z.string(),
  clone_url: z.string(),
  description: z.string().nullable(),
  private: z.boolean(),
  archived: z.boolean(),
  default_branch: z.string(),
}).loose();

export const ListGitHubRepositoriesResponseSchema = z.object({
  repositories: z.array(GitHubRepositorySchema).default([]),
  total_count: z.number().optional().default(0),
  next_page: z.number().nullable().optional().default(null),
}).loose();

export const EMPTY_LIST_GITHUB_REPOSITORIES_RESPONSE: ListGitHubRepositoriesResponse = {
  repositories: [],
  total_count: 0,
  next_page: null,
};

export const GitHubPullRequestSchema = z.object({
  id: z.string(),
  provider: z.string().optional().default("github"),
  workspace_id: z.string(),
  repo_owner: z.string(),
  repo_name: z.string(),
  number: z.number(),
  title: z.string(),
  state: z.string(),
  html_url: z.string(),
  branch: z.string().nullable(),
  author_login: z.string().nullable(),
  author_avatar_url: z.string().nullable(),
  merged_at: z.string().nullable(),
  closed_at: z.string().nullable(),
  pr_created_at: z.string(),
  pr_updated_at: z.string(),
  link_source: z.enum(["manual", "title", "branch", "auto"]).optional().catch(undefined),
  mergeable: z.string().nullable().optional(),
  merge_state_status: z.string().nullable().optional(),
  snapshot_available: z.boolean().optional(),
  checks_rollup: z.string().nullable().optional(),
  checks_conclusion: z.string().nullable().optional(),
  checks_total: z.number().optional().default(0),
  checks_passed: z.number().optional().default(0),
  checks_failed: z.number().optional().default(0),
  checks_running: z.number().optional().default(0),
  checks_pending: z.number().optional().default(0),
  failed_check_names: z.array(z.string()).optional().default([]),
  snapshot_stale: z.boolean().optional().default(false),
  snapshot_fetched_at: z.string().nullable().optional(),
  mergeable_state: z.string().nullable().optional(),
  additions: z.number().optional().default(0),
  deletions: z.number().optional().default(0),
  changed_files: z.number().optional().default(0),
}).loose();

// A malformed auto_complete block degrades to null (the issue page shows no
// automation line) instead of discarding the PR list with it.
export const PRAutoCompleteSchema = z.object({
  state: z.string(),
  pull_request_ids: z.array(z.string()).default([]),
  issue_disabled: z.boolean().default(false),
  workspace_enabled: z.boolean().default(true),
  target_status: z.string().optional().catch(undefined),
}).loose();

export const IssuePullRequestsResponseSchema = z.object({
  pull_requests: z.array(GitHubPullRequestSchema).default([]),
  auto_complete: PRAutoCompleteSchema.nullable().optional().default(null).catch(null),
}).loose();

export const EMPTY_ISSUE_PULL_REQUESTS_RESPONSE: IssuePullRequestsResponse = {
  pull_requests: [],
  auto_complete: null,
};

// Label responses are consumed by settings tables and resource pickers. Keep
// the resource type lenient so newer server scopes do not break older clients,
// while defaulting fields that predate scoped label catalogs.
export const LabelSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  resource_type: z.string().optional().default("issue"),
  name: z.string(),
  description: z.string().optional().default(""),
  color: z.string(),
  usage_count: z.number().optional().default(0),
  created_at: z.string(),
  updated_at: z.string(),
}).loose();

export const EMPTY_LABEL: Label = {
  id: "",
  workspace_id: "",
  resource_type: "issue",
  name: "",
  description: "",
  color: "#6b7280",
  usage_count: 0,
  created_at: "",
  updated_at: "",
};

export const ListLabelsResponseSchema = z.object({
  labels: z.array(LabelSchema).default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_LABELS_RESPONSE: ListLabelsResponse = {
  labels: [],
  total: 0,
};

// Issue status catalog (MUL-6243). `category` is parsed as a plain string
// rather than an enum: a newer server could in principle report a category this
// build does not know, and failing the whole catalog parse would leave the UI
// with no statuses at all. Consumers fall back to rendering by `color`/`name`
// when they do not recognize a category.
export const IssueStatusEntrySchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  key: z.string(),
  name: z.string(),
  description: z.string().optional().default(""),
  category: z.string().transform((value) => normalizeIssueStatusCategory(value) ?? value),
  color: z.string().optional().default("#6b7280"),
  icon: z.string().nullable().optional(),
  is_system: z.boolean().optional().default(false),
  position: z.number().optional().default(0),
  archived_at: z.string().nullable().optional().default(null),
  created_at: z.string(),
  updated_at: z.string(),
}).loose();

export const EMPTY_ISSUE_STATUS_ENTRY: IssueStatusEntry = {
  id: "",
  workspace_id: "",
  key: "",
  name: "",
  description: "",
  category: "unstarted",
  color: "#6b7280",
  is_system: false,
  position: 0,
  archived_at: null,
  created_at: "",
  updated_at: "",
};

export const ListIssueStatusesResponseSchema = z.object({
  statuses: z.array(IssueStatusEntrySchema).default([]),
  categories: z.array(z.string()).default([]).transform((values) => [...new Set(values.map((value) => normalizeIssueStatusCategory(value) ?? value))]),
  total: z.number().default(0),
}).loose();

// The fallback carries the four lifecycle categories. Concrete built-in status
// keys remain available through the status configuration.
export const EMPTY_LIST_ISSUE_STATUSES_RESPONSE: ListIssueStatusesResponse = {
  statuses: [],
  categories: ["unstarted", "started", "done", "closed"],
  total: 0,
};

export const ResourceLabelsResponseSchema = z.object({
  labels: z.array(LabelSchema).default([]),
  issue_revision: z.number().int().positive().optional(),
}).loose();

export const EMPTY_RESOURCE_LABELS_RESPONSE: ResourceLabelsResponse = {
  labels: [],
};

// Saved issue views (MUL-4796). `query`/`display` are opaque definition
// blobs interpreted client-side per `definition_version` — keep them as
// loose records so newer servers can add fields freely. `scope_type` /
// `visibility` stay lenient strings; downstream code uses explicit `===`
// comparisons and default branches per the API-compat rules.
export const IssueViewSchema = z.object({
  id: z.string(),
  workspace_id: z.string().default(""),
  owner_id: z.string().default(""),
  name: z.string().default(""),
  scope_type: z.string().default("workspace"),
  scope_id: z.string().nullish(),
  scope_variant: z.string().nullish(),
  visibility: z.string().default("private"),
  definition_version: z.number().default(1),
  query: z.record(z.string(), z.unknown()).default({}),
  display: z.record(z.string(), z.unknown()).default({}),
  revision: z.number().default(1),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export type IssueView = z.infer<typeof IssueViewSchema>;

export const IssueViewListSchema = z.array(IssueViewSchema);

export const IssueViewPreferenceSchema = z.object({
  scope_type: z.string().default("workspace"),
  scope_id: z.string().nullish(),
  prefs: z.object({
    hidden: z.array(z.string()).default([]),
    order: z.array(z.string()).default([]),
  }).loose().default({ hidden: [], order: [] }),
  updated_at: z.string().default(""),
}).loose();

export type IssueViewPreference = z.infer<typeof IssueViewPreferenceSchema>;

export const EMPTY_ISSUE_VIEW_PREFERENCE: IssueViewPreference = {
  scope_type: "workspace",
  scope_id: null,
  prefs: { hidden: [], order: [] },
  updated_at: "",
};

export interface CreateIssueViewRequest {
  name: string;
  scope_type: "workspace" | "my" | "project";
  scope_id?: string | null;
  scope_variant?: "assigned" | "created" | "involved" | "any" | "members" | "agents" | null;
  visibility: "private" | "workspace";
  definition_version: number;
  query: Record<string, unknown>;
  display: Record<string, unknown>;
}

// Custom property definitions. `type` stays a lenient string so newer server
// types don't break installed clients; UI narrows with isKnownPropertyType.
export const IssuePropertySchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  name: z.string(),
  type: z.string(),
  description: z.string().optional().default(""),
  icon: z.string().optional().default(""),
  config: z.object({
    options: z.array(z.object({
      id: z.string(),
      name: z.string(),
      color: z.string().optional().default("#6b7280"),
    }).loose()).optional(),
  }).loose().default({}),
  position: z.number().optional().default(0),
  archived: z.boolean().optional().default(false),
  archived_at: z.string().nullable().optional(),
  usage_count: z.number().optional().default(0),
  created_at: z.string(),
  updated_at: z.string(),
}).loose();

export const EMPTY_ISSUE_PROPERTY: IssueProperty = {
  id: "",
  workspace_id: "",
  name: "",
  type: "text",
  description: "",
  icon: "",
  config: {},
  position: 0,
  archived: false,
  usage_count: 0,
  created_at: "",
  updated_at: "",
};

// Quick actions (MUL-5465). `visibility` and `status` stay z.string() rather
// than z.enum: they are server-driven, and a newer server adding a value must
// degrade to the UI's default branch, not blank the whole list.
export const QuickActionSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  name: z.string(),
  description: z.string().optional().default(""),
  assignee_type: z.string(),
  assignee_id: z.string(),
  prompt: z.string().optional().default(""),
  visibility: z.string().optional().default("public"),
  status: z.string().optional().default("active"),
  last_used_at: z.string().nullable().optional().default(null),
  use_count: z.number().optional().default(0),
  created_by_id: z.string().optional().default(""),
  created_at: z.string(),
  updated_at: z.string(),
  target_name: z.string().optional(),
  // Both default to the pessimistic reading on an older server: "not known to
  // be public" and "not known to be missing" keep the settings row honest
  // rather than asserting a state the server never sent.
  target_public: z.boolean().optional().default(false),
  target_missing: z.boolean().optional().default(false),
}).loose();

export const EMPTY_QUICK_ACTION: QuickAction = {
  id: "",
  workspace_id: "",
  name: "",
  description: "",
  assignee_type: "agent",
  assignee_id: "",
  prompt: "",
  visibility: "public",
  status: "active",
  last_used_at: null,
  use_count: 0,
  created_by_id: "",
  created_at: "",
  updated_at: "",
  target_public: false,
  target_missing: true,
};

export const ListQuickActionsResponseSchema = z.object({
  quick_actions: z.array(QuickActionSchema).default([]),
}).loose();

export const EMPTY_LIST_QUICK_ACTIONS_RESPONSE: ListQuickActionsResponse = {
  quick_actions: [],
};

export const QuickActionRenderSchema = z.object({
  content: z.string().default(""),
}).loose();

export const ListPropertiesResponseSchema = z.object({
  properties: z.array(IssuePropertySchema).default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_PROPERTIES_RESPONSE: ListPropertiesResponse = {
  properties: [],
  total: 0,
};

// Value bag: keyed by definition UUID; values are primitives or string
// arrays (multi_select). The preprocess step drops entries with unknown
// shapes BEFORE validation — a newer server shipping an object-shaped value
// (future actor/relation types) must degrade to "that one property missing",
// never fail the whole IssueSchema and blank the list via parseWithFallback.
export const IssuePropertyValuesSchema = z.preprocess(
  (raw) => {
    if (typeof raw !== "object" || raw === null || Array.isArray(raw)) return {};
    const out: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(raw)) {
      const ok =
        typeof value === "string" ||
        typeof value === "number" ||
        typeof value === "boolean" ||
        (Array.isArray(value) && value.every((item) => typeof item === "string"));
      if (ok) out[key] = value;
    }
    return out;
  },
  z.record(z.string(), z.union([z.string(), z.number(), z.boolean(), z.array(z.string())])).default({}),
);

export const IssuePropertiesResponseSchema = z.object({
  properties: IssuePropertyValuesSchema,
  issue_revision: z.number().int().positive().optional(),
}).loose();

export const EMPTY_ISSUE_PROPERTIES_RESPONSE: IssuePropertiesResponse = {
  properties: {},
};

export interface AppConfigResponse {
  cdn_domain: string;
  // True when the CDN domain serves private content via time-bounded signed
  // URLs (CloudFront signing) — raw storage URLs on that domain are NOT
  // publicly fetchable and must not be used as native media sources
  // (MUL-3254). Older servers omit the field; treat that as false.
  cdn_signed?: boolean;
  allow_signup: boolean;
  google_client_id?: string;
  posthog_key?: string;
  posthog_host?: string;
  analytics_environment?: string;
  daemon_server_url?: string;
  daemon_app_url?: string;
  workspace_creation_disabled?: boolean;
  /** Whether this deployment offers the self-hosted Git provider integration
   * (self-host only; off on the managed cloud). Absent/false hides the whole
   * Settings → Integrations "Git providers" section. */
  vcs_integration_available?: boolean;
  feature_flags?: Record<string, boolean>;
  /** Whether this server understands local_directory `execution_mode` and
   * gates worktree mode at save time. Absent on every server that predates this
   * capability signal, which includes the ones that silently DROPPED an unknown
   * `execution_mode` and answered 201 — the resource then ran in place while
   * the user was promised isolation (#7113). Servers between that fix and this
   * signal do validate but cannot say so, and are treated as unable: the client
   * has no way to tell them apart, and only one of the two answers is safe. */
  local_worktree_supported?: boolean;
  /** Whether agent create/update persists `conversation_starters`. Older servers
   * silently ignored the unknown field, so absent must be treated as false. */
  agent_conversation_starters_supported?: boolean;
  /** Whether issue create atomically validates and persists `properties`.
   * Older servers silently ignore the field, so absent means unsupported. */
  issue_create_properties_supported?: boolean;
  /** Whether deleting a comment keeps its replies and the server routes
   * DELETE /api/comments/{id}/keep-replies. Older servers deleted the replies
   * too, so absent must be treated as false (#8296). */
  comment_delete_keep_replies_supported?: boolean;
  server_version?: string;
}

// ---------------------------------------------------------------------------
// Schemas for the highest-risk API endpoints — those whose responses drive
// the issue detail page (timeline, comments, subscribers) and the issues
// list. These are the surfaces that white-screened in #2143 / #2147 / #2192.
//
// These schemas are intentionally LENIENT:
//   - String enums are stored as `z.string()` rather than `z.enum([...])`.
//     A new server-side enum value should render as a generic fallback in
//     the UI, never crash a `safeParse`.
//   - Optional fields are unioned with `null` and given fallbacks where
//     existing UI code already coerces them.
//   - Arrays default to `[]` so a missing `reactions` / `attachments` /
//     `entries` field doesn't take the page down.
//   - Every object schema ends with `.loose()` so unknown server-side
//     fields pass through unchanged. zod 4's `.object()` defaults to STRIP,
//     which would silently delete fields the schema didn't explicitly list
//     — fine while the TS type doesn't claim them, but the moment a future
//     PR adds a TS field without updating the schema, the cast `as T` lies
//     and the field shows up as `undefined` at runtime. `.loose()` removes
//     that synchronisation hazard.
//
// These schemas are deliberately not typed as `z.ZodType<TimelineEntry>` /
// `z.ZodType<Issue>` etc. — the strict TS types narrow string fields to
// literal unions, which would defeat the leniency above. `parseWithFallback`
// returns the parsed value cast to the caller-supplied `T`, so the strict
// type still flows out at the call site; the schema only guards shape.
// ---------------------------------------------------------------------------

const ReactionSchema = z.object({
  id: z.string(),
  comment_id: z.string(),
  actor_type: z.string(),
  actor_id: z.string(),
  emoji: z.string(),
  created_at: z.string(),
});

// Nested attachments embedded in timeline/comment responses stay lenient on
// purpose: a single malformed attachment must not knock the whole timeline
// into the fallback `[]`.
const AttachmentSchema = z.object({
  id: z.string(),
}).loose();

const ChatQuickActionSchema = z.object({
  label: z.string(),
  prompt: z.string(),
  primary: z.boolean().optional(),
}).loose();

export const ChatMessageSchema = z.object({
  id: z.string(),
  chat_session_id: z.string(),
  role: z.enum(["user", "assistant"]).catch("assistant"),
  content: z.string().default(""),
  task_id: z.string().nullable().default(null),
  created_at: z.string().default(""),
  attachments: z.array(AttachmentSchema).optional(),
  failure_reason: z.string().nullable().optional(),
  elapsed_ms: z.number().nullable().optional(),
  message_kind: z
    .enum(["message", "no_response", "onboarding_kickoff", "onboarding_opening"])
    .catch("message")
    .optional(),
  // Optional additive data degrades independently: a malformed suggestion
  // must not hide the assistant reply that contains it.
  quick_actions: z.array(ChatQuickActionSchema).catch([]).optional().default([]),
}).loose();

export const ChatMessageListSchema = z.array(ChatMessageSchema).default([]);
export const EMPTY_CHAT_MESSAGE_LIST: ChatMessage[] = [];

export const ChatMessagesPageSchema = z.object({
  messages: z.array(ChatMessageSchema).default([]),
  limit: z.number().default(50),
  has_more: z.boolean().default(false),
  next_cursor: z.object({
    created_at: z.string(),
    id: z.string(),
  }).loose().nullable().optional(),
}).loose();

// Standalone attachment lookup (`GET /api/attachments/{id}`) is the source of
// truth for click-time download URLs. The two fields the download flow opens
// in a new tab — `download_url` and `url` — must be strings, otherwise we'd
// happily `window.open(undefined)`. `filename` gates the toast/title and is
// also enforced so a missing value falls back to the empty record below.
//
// `markdown_url` is parsed lenient: a server old enough to predate
// MUL-3192 omits the field, in which case the schema defaults it to "".
// Callers that need to persist a URL into markdown should go through the
// `useFileUpload` helper (which falls back to the legacy
// `attachmentDownloadPath` shape when `markdown_url` is empty), so the
// empty-string default does not silently break any persistence path.
export const AttachmentResponseSchema = z.object({
  id: z.string(),
  url: z.string(),
  download_url: z.string(),
  // Forced-attachment ("download button") URL — credential-free and, unlike
  // `download_url`, always Content-Disposition: attachment across every storage
  // mode. Optional: a server older than this field omits it, and callers fall
  // back to `download_url` / the stable endpoint. Never persisted (short-lived).
  attachment_download_url: z.string().optional(),
  markdown_url: z.string().optional().default(""),
  filename: z.string(),
  chat_session_id: z.string().nullable().optional(),
  chat_message_id: z.string().nullable().optional(),
}).loose();

export const EMPTY_ATTACHMENT: Attachment = {
  id: "",
  workspace_id: "",
  issue_id: null,
  comment_id: null,
  chat_session_id: null,
  chat_message_id: null,
  uploader_type: "",
  uploader_id: "",
  filename: "",
  url: "",
  download_url: "",
  markdown_url: "",
  content_type: "",
  size_bytes: 0,
  created_at: "",
};

// All object schemas use `.loose()` so unknown server-side fields pass
// through unchanged. zod 4's `.object()` defaults to STRIP, which would
// silently drop new fields and surface as a "field neither showed up in
// the UI" mystery the next time the TS type adopted them but the schema
// wasn't updated in lock-step. `.loose()` removes that synchronisation
// hazard — the schema validates the shape it knows about and leaves the
// rest alone.
// One receipt per running turn a comment steered. A malformed receipt is
// dropped on its own rather than failing the whole comment.
const CommentSupplementReceiptSchema = z.object({
  task_id: z.string(),
  agent_id: z.string().optional().catch(undefined),
  status: z.enum(["pending", "delivering", "delivered", "failed"]),
  failure_reason: z.string().optional().catch(undefined),
  delivered_at: z.string().optional().catch(undefined),
});

const CommentSupplementReceiptsSchema = z.array(z.unknown()).optional().catch(undefined)
  .transform((raw) => raw?.flatMap((item) => {
    const parsed = CommentSupplementReceiptSchema.safeParse(item);
    return parsed.success ? [parsed.data] : [];
  }));

const TimelineEntrySchema = z.object({
  type: z.string(),
  id: z.string(),
  actor_type: z.string(),
  actor_id: z.string(),
  created_at: z.string(),
  actor_name: z.string().optional(),
  actor_avatar_url: z.string().optional(),
  action: z.string().optional(),
  details: z.record(z.string(), z.unknown()).optional(),
  content: z.string().optional(),
  parent_id: z.string().nullable().optional(),
  updated_at: z.string().optional(),
  revision: z.number().int().positive().optional(),
  comment_type: z.string().optional(),
  reactions: z.array(ReactionSchema).optional(),
  attachments: z.array(AttachmentSchema).optional(),
  source_task_id: z.string().nullable().optional(),
  supplements: CommentSupplementReceiptsSchema,
  supplement_task_id: z.string().optional().catch(undefined),
  supplement_status: z.enum(["pending", "delivering", "delivered", "failed"]).optional().catch(undefined),
  supplement_failure_reason: z.string().optional().catch(undefined),
  supplement_delivered_at: z.string().optional().catch(undefined),
  // Tombstone marker (#8296). Lenient: a malformed value reads as a live
  // comment instead of failing the whole timeline.
  deleted_at: z.string().nullable().optional().catch(undefined),
  coalesced_count: z.number().optional(),
}).loose();

// /timeline returns a flat array of TimelineEntry, oldest first. The
// previously cursor-paginated wrapper was removed (#1929) — at observed data
// sizes (p99 ~30 entries per issue) paged delivery only created bugs.
export const TimelineEntriesSchema = z.array(TimelineEntrySchema);

export const EMPTY_TIMELINE_ENTRIES: TimelineEntry[] = [];

const OptionalStringSchema = z.preprocess(
  (value) => (typeof value === "string" ? value : undefined),
  z.string().optional(),
);

const BooleanWithDefaultSchema = (fallback: boolean) =>
  z.preprocess(
    (value) => (typeof value === "boolean" ? value : undefined),
    z.boolean().default(fallback),
  );

const FeatureFlagsSchema = z.preprocess(
  (value) =>
    value && typeof value === "object" && !Array.isArray(value)
      ? value
      : undefined,
  z.record(z.string(), BooleanWithDefaultSchema(false)).default({}),
);

/**
 * POST /api/auth/refresh — sliding session renewal (MUL-7436).
 *
 * `token` is present only for clients that carry the session as a string
 * (Desktop, mobile). A cookie-authenticated browser gets the renewed session
 * as a Set-Cookie and must never be handed a readable JWT, so the field is
 * absent there rather than empty.
 *
 * `renewed: false` is the normal answer to asking early, not an error —
 * clients poll on a cadence and most calls land outside the renewal window.
 * `check_again_in_seconds` is that cadence: the server derives it from the
 * deployment's configured TTL, so no client hardcodes one.
 */
export interface RefreshSessionResponse {
  token?: string;
  expires_at: string;
  renewed: boolean;
  check_again_in_seconds: number;
}

export const RefreshSessionResponseSchema = z.object({
  token: OptionalStringSchema,
  expires_at: OptionalStringSchema,
  renewed: BooleanWithDefaultSchema(false),
  check_again_in_seconds: z.number().int().nonnegative().default(0),
}).loose();

// Fail closed: an unreadable response means "nothing was renewed", so the
// client keeps the session it already has and retries later. A fallback that
// claimed renewal would drop a working session on the floor.
export const EMPTY_REFRESH_SESSION_RESPONSE: RefreshSessionResponse = {
  expires_at: "",
  renewed: false,
  check_again_in_seconds: 0,
};

export const AppConfigSchema = z.object({
  cdn_domain: z.string().default(""),
  cdn_signed: BooleanWithDefaultSchema(false),
  allow_signup: BooleanWithDefaultSchema(true),
  google_client_id: OptionalStringSchema,
  posthog_key: OptionalStringSchema,
  posthog_host: OptionalStringSchema,
  analytics_environment: OptionalStringSchema,
  daemon_server_url: OptionalStringSchema,
  daemon_app_url: OptionalStringSchema,
  workspace_creation_disabled: BooleanWithDefaultSchema(false).optional(),
  vcs_integration_available: BooleanWithDefaultSchema(false).optional(),
  feature_flags: FeatureFlagsSchema,
  local_worktree_supported: BooleanWithDefaultSchema(false),
  agent_conversation_starters_supported: BooleanWithDefaultSchema(false),
  issue_create_properties_supported: BooleanWithDefaultSchema(false),
  comment_delete_keep_replies_supported: BooleanWithDefaultSchema(false),
  server_version: OptionalStringSchema,
}).loose();

export const EMPTY_APP_CONFIG: AppConfigResponse = {
  cdn_domain: "",
  cdn_signed: false,
  allow_signup: true,
  google_client_id: "",
  daemon_server_url: "",
  daemon_app_url: "",
  workspace_creation_disabled: false,
  vcs_integration_available: false,
  // Fail closed: an unreadable config must not look like a server that
  // validates execution_mode.
  local_worktree_supported: false,
  // Fail closed: old servers returned success while dropping the field.
  agent_conversation_starters_supported: false,
  // Fail closed: old servers returned success while dropping create properties.
  issue_create_properties_supported: false,
  // Fail closed: old servers delete a comment's replies with it.
  comment_delete_keep_replies_supported: false,
  feature_flags: {},
};

// Preference keys may grow over time, so keep both the key and value spaces
// forward-compatible while still rejecting non-string persisted data.
export const NotificationPreferenceResponseSchema = z.object({
  workspace_id: z.string(),
  preferences: z.record(z.string(), z.string()).default({}),
}).loose();

export const EMPTY_NOTIFICATION_PREFERENCE_RESPONSE: NotificationPreferenceResponse = {
  workspace_id: "",
  preferences: {},
};

export const CreateFeedbackResponseSchema = z.object({
  id: z.string(),
  created_at: z.string(),
}).loose();

export const EMPTY_CREATE_FEEDBACK_RESPONSE: CreateFeedbackResponse = {
  id: "",
  created_at: "",
};

export const CommentSchema = z.object({
  id: z.string(),
  issue_id: z.string(),
  author_type: z.string(),
  author_id: z.string(),
  content: z.string(),
  type: z.string(),
  parent_id: z.string().nullable(),
  reactions: z.array(ReactionSchema).default([]),
  attachments: z.array(AttachmentSchema).default([]),
  created_at: z.string(),
  updated_at: z.string(),
  revision: z.number().int().positive().optional(),
  source_task_id: z.string().nullable().optional(),
  supplements: CommentSupplementReceiptsSchema,
  supplement_task_id: z.string().optional().catch(undefined),
  supplement_status: z.enum(["pending", "delivering", "delivered", "failed"]).optional().catch(undefined),
  supplement_failure_reason: z.string().optional().catch(undefined),
  supplement_delivered_at: z.string().optional().catch(undefined),
  // Set only on comments a quick action produced (MUL-5465). Server-only.
  quick_action_id: z.string().nullable().optional(),
  deleted_at: z.string().nullable().optional().catch(undefined),
}).loose();

export const CommentsListSchema = z.array(CommentSchema);

// Degraded placeholder for a comment response that failed schema validation.
// The empty id is the caller's signal that nothing usable came back — the run
// UI treats it as "could not read the result" rather than a successful run.
export const EMPTY_COMMENT: Comment = {
  id: "",
  issue_id: "",
  author_type: "member",
  author_id: "",
  content: "",
  type: "comment",
  parent_id: null,
  reactions: [],
  attachments: [],
  created_at: "",
  updated_at: "",
  resolved_at: null,
  resolved_by_type: null,
  resolved_by_id: null,
};

const CommentTriggerPreviewAgentSchema = z.object({
  id: z.string(),
  name: z.string().default(""),
  avatar_url: z.string().optional(),
  source: z.string().default(""),
  reason: z.string().default(""),
}).loose();

// Per-target outcome of an explicit @agent / @squad mention (MUL-4525 §2).
// target_id is required to correlate with the client's rendered mention; a
// malformed entry (missing id) is dropped rather than failing the whole payload.
export const CommentTriggerOutcomeSchema = z.object({
  target_type: z.string().default(""),
  target_id: z.string(),
  status: z.string().default(""),
  reason_code: z.string().default(""),
}).loose();

export const CommentTriggerPreviewSchema = z.object({
  agents: z.array(CommentTriggerPreviewAgentSchema).default([]),
  // Drop malformed blocked entries INDIVIDUALLY (MUL-4525): a single bad item
  // must not discard the whole set of valid blocked mentions. A non-array
  // degrades to []; each valid entry is kept, each malformed one dropped.
  blocked: z
    .array(z.unknown())
    .catch([])
    .default([])
    .transform((items) =>
      items.flatMap((item) => {
        const parsed = CommentTriggerOutcomeSchema.safeParse(item);
        return parsed.success ? [parsed.data] : [];
      }),
    ),
}).loose();

const IssueTriggerPreviewItemSchema = z.object({
  issue_id: z.string(),
  agent_id: z.string().default(""),
  source: z.string().default(""),
}).loose();

export const IssueTriggerPreviewSchema = z.object({
  triggers: z.array(IssueTriggerPreviewItemSchema).default([]),
  total_count: z.number().default(0),
}).loose();

// Metadata is primitive-only by API/DB contract. Stay lenient on shape:
// unknown keys land as `unknown` to a caller, but the field itself defaults
// to {} so consumers never need to nil-guard `issue.metadata`.
const IssueMetadataSchema = z.record(z.string(), z.union([z.string(), z.number(), z.boolean()])).default({});

const SourceContextAttachmentSchema = z.object({
  id: z.string(),
  source_attachment_id: z.string().optional(),
  owner_type: z.string(),
  owner_id: z.string(),
  filename: z.string(),
  content_type: z.string(),
  size_bytes: z.number(),
  created_at: z.string(),
}).loose();

// Early source-context servers encoded an empty Go slice as JSON null. Keep
// installed clients compatible while normalizing consumers onto the canonical
// array shape emitted by current servers.
const SourceContextAttachmentsSchema = z.array(SourceContextAttachmentSchema)
  .nullish()
  .transform((attachments) => attachments ?? []);

const SourceContextAuthorSchema = z.object({
  type: z.string(),
  id: z.string(),
  name: z.string(),
}).loose();

const SourceContextIssueSnapshotSchema = z.object({
  id: z.string(),
  identifier: z.string(),
  number: z.number(),
  title: z.string(),
  description: z.string().nullable(),
  created_at: z.string(),
  updated_at: z.string(),
  revision: z.number(),
  attachments: SourceContextAttachmentsSchema,
}).loose();

const SourceContextCommentSnapshotSchema = z.object({
  id: z.string(),
  parent_id: z.string().nullable(),
  type: z.string(),
  content: z.string(),
  author: SourceContextAuthorSchema,
  created_at: z.string(),
  updated_at: z.string(),
  revision: z.number(),
  attachments: SourceContextAttachmentsSchema,
  deleted: z.boolean().optional().catch(undefined),
}).loose();

export const SourceContextSnapshotSchema = z.object({
  version: z.number().optional(),
  captured_by_user_id: z.string().optional(),
  captured_at: z.string().optional(),
  source_issue: SourceContextIssueSnapshotSchema,
  comment_thread: z.array(SourceContextCommentSnapshotSchema),
  anchor_comment_id: z.string(),
}).loose();

export const SourceContextPreviewSchema = SourceContextSnapshotSchema.extend({
  capture_token: z.string().min(1),
  limits: z.object({
    comment_count: z.number(),
    text_bytes: z.number(),
    attachment_count: z.number(),
    attachment_bytes: z.number(),
  }).loose(),
}).loose();

const IssueSourceContextSchema = z.object({
  id: z.string(),
  version: z.number(),
  usage: z.string(),
  captured_at: z.string(),
  display_state: z.string(),
  source_issue_state: z.string(),
  comment_thread_state: z.string(),
  anchor_comment_state: z.string(),
  can_open_current_source: z.boolean(),
  change_reasons: z.array(z.string()).optional(),
  change_details: z.object({
    changed_comment_ids: z.array(z.string()),
    added_comments: z.array(SourceContextCommentSnapshotSchema).optional(),
    removed_comment_ids: z.array(z.string()).optional(),
    description_attachment_changes: z.array(z.object({
      kind: z.string(),
      attachment_id: z.string(),
      filename: z.string(),
      previous_filename: z.string().optional(),
    }).loose()),
  }).loose().optional(),
  current_source: z.object({
    issue_id: z.string(),
    identifier: z.string(),
    anchor_comment_id: z.string(),
  }).loose().optional(),
  source_author_state: z.array(z.object({
    type: z.string(),
    id: z.string(),
    captured_name: z.string(),
    current_name: z.string().optional(),
    state: z.string(),
  }).loose()).optional(),
  snapshot: SourceContextSnapshotSchema,
}).loose();

export const CommentSubIssueTaskResponseSchema = z.object({
  task_id: z.string().min(1),
}).loose();

export const IssueSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  number: z.number(),
  identifier: z.string(),
  title: z.string(),
  description: z.string().nullable(),
  status: z.string(),
  // The canonical status whose platform behavior `status` carries — equal to
  // `status` for the 7 built-ins, and the inherited category for a custom
  // status. Optional because only endpoints that resolve it emit it, so
  // consumers must fall back to `status` rather than treat "" as a category.
  // (MUL-6243)
  status_category: z.string().transform((value) => normalizeIssueStatusCategory(value) ?? value).optional(),
  // A CUSTOM status's display name; "" for a built-in, which clients localize
  // from the key. Optional so a response from a server that predates the field
  // still validates.
  //
  // .catch(undefined) because this is ADDITIVE display data and the failure it
  // guards against is disproportionate: a parse failure anywhere in IssueSchema
  // takes the whole response through parseWithFallback to EMPTY_LIST_ISSUES_RESPONSE,
  // so one malformed name from a mixed-version deploy would blank an entire
  // issue list. Same treatment as source_context and labels above. Nothing reads
  // this field to make a decision — useStatusLabel resolves the label from the
  // catalog — so dropping it costs a fallback to the key. (MUL-6749)
  status_name: z.string().optional().catch(undefined),
  priority: z.string(),
  assignee_type: z.string().nullable(),
  assignee_id: z.string().nullable(),
  creator_type: z.string(),
  creator_id: z.string(),
  parent_issue_id: z.string().nullable(),
  // Additive pointer (MUL-7349); same disproportionate-failure reasoning as
  // status_name above, and older backends do not send it at all.
  duplicate_of: z
    .object({ id: z.string(), identifier: z.string(), title: z.string(), status: z.string() })
    .nullable()
    .optional()
    .catch(undefined),
  project_id: z.string().nullable(),
  type: z.enum(ISSUE_TYPES).nullable().catch(null),
  worklane_id: z.string().nullable().default(null),
  release_id: z.string().nullable().default(null),
  done_at: z.string().nullable().default(null),
  position: z.number(),
  // Older backends predate `stage`; default to null so a missing field parses
  // cleanly into the non-optional Issue.stage (number | null).
  stage: z.number().nullable().default(null),
  // Absent on Workspace default, for agents and on older backends.
  agent_questions: z.boolean().nullable().default(null).catch(null),
  start_date: z.string().nullable(),
  due_date: z.string().nullable(),
  metadata: IssueMetadataSchema,
  // Older backends predate custom properties; default {} so consumers never
  // nil-guard issue.properties.
  properties: IssuePropertyValuesSchema,
  reactions: z.array(z.unknown()).optional(),
  labels: z.array(z.unknown()).optional(),
  created_at: z.string(),
  updated_at: z.string(),
  revision: z.number().int().positive().optional(),
  // Optional for compatibility with older self-hosted backends; a current
  // backend emits null until its historical backfill reaches the issue.
  last_activity_at: z.string().nullable().optional(),
  // Detail-only and potentially large. A malformed additive field must not
  // erase an otherwise usable issue returned by a mixed-version server.
  source_context: IssueSourceContextSchema.optional().catch(undefined),
}).loose();

export const ListIssuesResponseSchema = z.object({
  issues: z.array(IssueSchema).default([]),
  total: z.number().default(0),
}).loose();

// Response schema for POST /api/issues. Two tightenings over IssueSchema:
//
//   - `id` must be non-empty. A created issue always carries a real id, so an
//     empty/absent id means the create effectively failed. createIssue turns a
//     schema failure into a rejection (not a fabricated success), so tightening
//     id here routes an id-less body to that same failure path.
//   - `labels` is the backend-compatibility signal the create modal reads to
//     decide whether the backend attached labels in the create transaction
//     (present) or predates that (absent → fall back to per-label attach).
//     Validate it strictly as Label[] and degrade a malformed value to
//     `undefined` — the same as an absent field — so a wrong shape (null,
//     object, a garbage array) can never masquerade as "handled" and suppress
//     the fallback. Unlike the loose IssueSchema.labels (z.array(z.unknown())),
//     the elements are fully validated. See packages/views/modals/create-issue.tsx.
export const CreateIssueResponseSchema = IssueSchema.extend({
  id: z.string().min(1),
  labels: z.array(LabelSchema).optional().catch(undefined),
}).loose();

export const EMPTY_LIST_ISSUES_RESPONSE: ListIssuesResponse = {
  issues: [],
  total: 0,
};

const SearchIssueResultSchema = IssueSchema.extend({
  match_source: z.string(),
  matched_snippet: z.string().optional(),
  matched_description_snippet: z.string().optional(),
  matched_comment_snippet: z.string().optional(),
}).loose();

export const SearchIssuesResponseSchema = z.object({
  issues: z.array(SearchIssueResultSchema).default([]),
}).loose();

export const EMPTY_SEARCH_ISSUES_RESPONSE: SearchIssuesResponse = {
  issues: [],
};

export const ProjectSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  title: z.string(),
  description: z.string().nullable(),
  icon: z.string().nullable(),
  status: z.enum(["planned", "in_progress", "paused", "completed", "cancelled"]).catch("planned"),
  priority: z.enum(["urgent", "high", "medium", "low", "none"]).catch("none"),
  lead_type: z.enum(["member", "agent"]).nullable().catch(null),
  lead_id: z.string().nullable(),
  // .default(null) so a project from a backend before organisations parses as
  // having none instead of failing the object and blanking its batch.
  organisation_id: z.string().nullable().default(null),
  // .default(null) so a project from an older backend (frontend deploys before
  // backend) that omits these keys parses to null instead of failing the whole
  // object — which would degrade a search/list batch to the empty fallback.
  start_date: z.string().nullable().default(null),
  due_date: z.string().nullable().default(null),
  created_at: z.string(),
  updated_at: z.string(),
  issue_count: z.number().default(0),
  done_count: z.number().default(0),
  resource_count: z.number().default(0),
}).loose();

export const EMPTY_PROJECT: Project = {
  id: "",
  workspace_id: "",
  title: "",
  description: null,
  icon: null,
  status: "planned",
  priority: "none",
  lead_type: null,
  lead_id: null,
  organisation_id: null,
  start_date: null,
  due_date: null,
  created_at: "",
  updated_at: "",
  issue_count: 0,
  done_count: 0,
  resource_count: 0,
};

export const ListProjectsResponseSchema = z.object({
  projects: z.array(ProjectSchema).default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_PROJECTS_RESPONSE: ListProjectsResponse = {
  projects: [],
  total: 0,
};

const SearchProjectResultSchema = ProjectSchema.extend({
  match_source: z.string(),
  matched_snippet: z.string().optional(),
}).loose();

export const SearchProjectsResponseSchema = z.object({
  projects: z.array(SearchProjectResultSchema).default([]),
}).loose();

export const EMPTY_SEARCH_PROJECTS_RESPONSE: SearchProjectsResponse = {
  projects: [],
};

// Organisations group the Projects page and back their own pages. The identity
// fields and the key stay required, since an organisation without them cannot
// be shown, grouped or given folders: a malformed row is dropped from the list
// without hiding the other organisations, and a malformed single response
// degrades to EMPTY_ORGANISATION, whose empty id reads as not found.
export const OrganisationSchema: z.ZodType<Organisation> = z.object({
  id: z.string(),
  workspace_id: z.string(),
  name: z.string(),
  key: z.string(),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_ORGANISATION: Organisation = {
  id: "",
  workspace_id: "",
  name: "",
  key: "",
  created_at: "",
  updated_at: "",
};

export const ListOrganisationsResponseSchema = z.object({
  organisations: z
    .array(OrganisationSchema.catch(EMPTY_ORGANISATION))
    .transform((organisations) => organisations.filter((organisation) => organisation.id !== ""))
    .default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_ORGANISATIONS_RESPONSE: ListOrganisationsResponse = {
  organisations: [],
  total: 0,
};

// Repositories of a project (ADR-0006). The identity fields and `full_name`
// stay required, since a repository without them cannot be shown or linked: a
// malformed row is dropped from the list without hiding the others. Every
// other field has a default, and an unknown `version_policy` from a newer
// server reads as `none`, so the UI never offers a release flow it cannot read.
const VERSION_POLICIES = [
  "pubspec.yaml",
  "Cargo.toml",
  "pyproject.toml",
  "package.json",
  "tags",
  "none",
] as const satisfies readonly VersionPolicy[];

export const RepositorySchema: z.ZodType<Repository> = z.object({
  id: z.string(),
  workspace_id: z.string(),
  project_id: z.string(),
  created_by: z.string().nullable().default(null),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
  github_repo_id: z.number().default(0),
  full_name: z.string(),
  private: z.boolean().default(false),
  default_branch: z.string().default(""),
  checkout_daemon_id: z.string().default(""),
  checkout_path: z.string().default(""),
  version_policy: z.enum(VERSION_POLICIES).catch("none"),
  // null while not found yet, so a repository from a server before install
  // commands starts the check that finds it.
  install_command: z.string().nullable().default(null),
  last_tag: z.string().nullable().default(null),
}).loose();

export const EMPTY_REPOSITORY: Repository = {
  id: "",
  workspace_id: "",
  project_id: "",
  created_by: null,
  created_at: "",
  updated_at: "",
  github_repo_id: 0,
  full_name: "",
  private: false,
  default_branch: "",
  checkout_daemon_id: "",
  checkout_path: "",
  version_policy: "none",
  install_command: null,
  last_tag: null,
};

export const ListRepositoriesResponseSchema = z.object({
  repositories: z
    .array(RepositorySchema.catch(EMPTY_REPOSITORY))
    .transform((repositories) => repositories.filter((repository) => repository.id !== ""))
    .default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_REPOSITORIES_RESPONSE: ListRepositoriesResponse = {
  repositories: [],
  total: 0,
};

const EMPTY_AVAILABLE_REPOSITORY: AvailableRepository = {
  github_repo_id: 0,
  full_name: "",
  private: false,
  default_branch: "",
};

const AvailableRepositorySchema: z.ZodType<AvailableRepository> = z.object({
  github_repo_id: z.number().default(0),
  full_name: z.string(),
  private: z.boolean().default(false),
  default_branch: z.string().default(""),
}).loose();

// A malformed entry is dropped, and a missing list reads as empty, so the Add
// repository dialog still offers "Missing one? Give Multica access on GitHub".
export const AvailableRepositoriesResponseSchema = z.object({
  repositories: z
    .array(AvailableRepositorySchema.catch(EMPTY_AVAILABLE_REPOSITORY))
    .transform((repositories) => repositories.filter((repository) => repository.full_name !== ""))
    .default([]),
  install_url: z.string().default(""),
}).loose();

export const EMPTY_AVAILABLE_REPOSITORIES_RESPONSE: ListAvailableRepositoriesResponse = {
  repositories: [],
  install_url: "",
};

const RepositoryCheckoutFailureSchema = z.object({
  code: z
    .enum(["not_found", "not_git", "origin_mismatch", "wrong_branch", "clone_failed", "unknown"])
    .catch("unknown"),
  detail: z.string().catch(""),
}).loose();

// The checkout check is polled like a runtime directory listing: every field
// degrades on its own, an unknown failure code is kept as `unknown` (a failed
// check the dialog still explains with the server's detail), and a missing
// `failures` reads as none.
export const RepositoryCheckoutRequestSchema = z.object({
  id: z.string().catch(""),
  runtime_id: z.string().catch(""),
  status: z.enum(["pending", "running", "completed", "failed", "timeout"]).catch("failed"),
  resolved_path: z.string().catch(""),
  passed: z.boolean().catch(false),
  failures: z.array(RepositoryCheckoutFailureSchema).catch([]),
  version_files: z.array(z.string()).catch([]),
  last_tag: z.string().catch(""),
  install_commands: z
    .array(z.object({ file: z.string().catch(""), command: z.string().catch("") }).loose())
    .catch([]),
  error: z.string().catch(""),
  created_at: z.string().catch(""),
  updated_at: z.string().catch(""),
}).loose();

// `failed` is the honest fallback for an unparseable check: `completed` without
// `passed` would read as a check that ran and failed for no reason, and
// `pending` would poll until the client-side timeout.
export const MALFORMED_REPOSITORY_CHECKOUT_REQUEST: RepositoryCheckoutRequest = {
  id: "",
  runtime_id: "",
  status: "failed",
  resolved_path: "",
  passed: false,
  failures: [],
  version_files: [],
  last_tag: "",
  install_commands: [],
  error: "invalid repository checkout response",
  created_at: "",
  updated_at: "",
};

// Worklanes of a repository. A malformed row is dropped from the list, as for
// repositories.
const WorklaneIssueRefSchema = z.object({
  id: z.string(),
  identifier: z.string(),
  title: z.string().default(""),
  status: z.string().default(""),
}).loose();

export const WorklaneSchema: z.ZodType<Worklane> = z.object({
  id: z.string(),
  workspace_id: z.string(),
  repository_id: z.string(),
  name: z.string(),
  folder: z.string().nullable().default(null),
  daemon_id: z.string().nullable().default(null),
  path: z.string().nullable().default(null),
  is_main: z.boolean().default(false),
  position: z.number().default(0),
  // `free` is the fallback: an unknown state never reads as work in progress.
  state: z.enum(["free", "busy"]).catch("free"),
  branch: z.string().nullable().default(null),
  active_issue: WorklaneIssueRefSchema.nullable().default(null).catch(null),
  next_issue: WorklaneIssueRefSchema.nullable().default(null).catch(null),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_WORKLANE: Worklane = {
  id: "",
  workspace_id: "",
  repository_id: "",
  name: "",
  folder: null,
  daemon_id: null,
  path: null,
  is_main: false,
  position: 0,
  state: "free",
  branch: null,
  active_issue: null,
  next_issue: null,
  created_at: "",
  updated_at: "",
};

export const ListWorklanesResponseSchema = z.object({
  worklanes: z
    .array(WorklaneSchema.catch(EMPTY_WORKLANE))
    .transform((worklanes) => worklanes.filter((worklane) => worklane.id !== ""))
    .default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_WORKLANES_RESPONSE: ListWorklanesResponse = {
  worklanes: [],
  total: 0,
};

const WorklaneRequestStepSchema = z.object({
  key: z.enum(["fetch", "add_worktree", "install", "open", "check", "switch", "unknown"]).catch("unknown"),
  state: z.enum(["pending", "running", "done", "skipped", "failed", "unknown"]).catch("unknown"),
  detail: z.string().catch(""),
  output: z.string().catch(""),
  duration_ms: z.number().catch(0),
}).loose();

const WORKLANE_REQUEST_REASON_CODES = [
  "name_required",
  "name_taken",
  "folder_invalid",
  "folder_taken",
  "runtime_no_checkout",
  "runtime_offline",
  "runtime_cannot_make_worktrees",
  "install_command_unknown",
  "request_running",
  "main_fixed",
  "path_issue_worktree",
  "path_not_checkout",
  "lost",
  "busy",
  "uncommitted",
  "unpushed",
  "stashed",
  "unknown",
] as const satisfies readonly WorklaneRequestReasonCode[];

const WorklaneRequestReasonSchema = z.object({
  code: z.enum(WORKLANE_REQUEST_REASON_CODES).catch("unknown"),
  detail: z.string().catch(""),
  count: z.number().optional().catch(undefined),
}).loose();

// A worklane request is polled every second while it runs. Every field
// degrades on its own: an unknown step key, step state, status or reason code
// from a newer server is kept as `unknown`, which the dialog shows as a failed
// step with the server's detail, and a missing `steps` reads as none.
export const WorklaneRequestSchema: z.ZodType<WorklaneRequest> = z.object({
  id: z.string().catch(""),
  kind: z.enum(["create", "remove", "switch", "unknown"]).catch("unknown"),
  repository_id: z.string().catch(""),
  worklane_id: z.string().nullable().catch(null),
  name: z.string().catch(""),
  folder: z.string().catch(""),
  path: z.string().catch(""),
  base: z.string().catch(""),
  install_command: z.string().catch(""),
  daemon_id: z.string().catch(""),
  runtime_name: z.string().catch(""),
  status: z.enum(["running", "failed", "completed", "refused", "unknown"]).catch("unknown"),
  steps: z.array(WorklaneRequestStepSchema).catch([]),
  reason: WorklaneRequestReasonSchema.nullable().catch(null),
  created_by: z.string().nullable().catch(null),
}).loose();

// `failed` is the honest fallback for an unparseable request: the dialog stops
// polling and offers Try again instead of spinning or claiming success.
export const MALFORMED_WORKLANE_REQUEST: WorklaneRequest = {
  id: "",
  kind: "unknown",
  repository_id: "",
  worklane_id: null,
  name: "",
  folder: "",
  path: "",
  base: "",
  install_command: "",
  daemon_id: "",
  runtime_name: "",
  status: "failed",
  steps: [],
  reason: { code: "unknown", detail: "invalid worklane request response" },
  created_by: null,
};

const WorklaneActivityShape = {
  id: z.string().catch(""),
  actor_type: z.string().catch(""),
  actor_id: z.string().nullable().catch(null),
  created_at: z.string().catch(""),
};

const UNKNOWN_WORKLANE_ACTIVITY_ENTRY: WorklaneActivityEntry = {
  id: "",
  actor_type: "",
  actor_id: null,
  created_at: "",
  action: "unknown",
  details: {},
};

const ReleaseVersionChangeSchema = z.object({
  release_id: z.string().catch(""),
  title: z.string().catch(""),
  from: z.string().catch(""),
  to: z.string().catch(""),
}).loose();

const ReleaseActivityDetailsShape = {
  release_id: z.string().catch(""),
  version: z.string().catch(""),
  title: z.string().catch(""),
};

const EmptyReleaseActivityDetails = { release_id: "", version: "", title: "" };

// An entry of a known action parses its details, each field with a default.
// Any other action, from a newer server, is kept as `unknown` and not
// rendered, so an older desktop app ignores the entries it cannot word.
const WorklaneActivityEntrySchema: z.ZodType<WorklaneActivityEntry> = z.union([
  z.object({
    ...WorklaneActivityShape,
    action: z.literal("worklane_created"),
    details: z
      .object({ name: z.string().catch(""), runtime_name: z.string().catch("") })
      .loose()
      .catch({ name: "", runtime_name: "" }),
  }).loose(),
  z.object({
    ...WorklaneActivityShape,
    action: z.literal("worklane_renamed"),
    details: z
      .object({ from: z.string().catch(""), to: z.string().catch("") })
      .loose()
      .catch({ from: "", to: "" }),
  }).loose(),
  z.object({
    ...WorklaneActivityShape,
    action: z.literal("release_planned"),
    details: z.object(ReleaseActivityDetailsShape).loose().catch(EmptyReleaseActivityDetails),
  }).loose(),
  z.object({
    ...WorklaneActivityShape,
    action: z.literal(["release_edited", "release_removed"]),
    details: z.object({
      ...ReleaseActivityDetailsShape,
      changes: z.array(ReleaseVersionChangeSchema).catch([]),
    }).loose().catch({ ...EmptyReleaseActivityDetails, changes: [] }),
  }).loose(),
  z.object({
    ...WorklaneActivityShape,
    action: z.literal("release_moved"),
    details: z.object({
      ...ReleaseActivityDetailsShape,
      direction: z.enum(["earlier", "later", "unknown"]).catch("unknown"),
      changes: z.array(ReleaseVersionChangeSchema).catch([]),
    }).loose().catch({ ...EmptyReleaseActivityDetails, direction: "unknown", changes: [] }),
  }).loose(),
  z.object({
    ...WorklaneActivityShape,
    action: z.literal("worklane_switched"),
    details: z.object({
      from: z.string().nullable().catch(null),
      to: z.string().nullable().catch(null),
      issue_id: z.string().nullable().catch(null),
    }).loose().catch({ from: null, to: null, issue_id: null }),
  }).loose(),
  z.object({
    ...WorklaneActivityShape,
    action: z.literal("worklane_kept"),
    details: z.object({
      branch: z.string().catch(""),
      reason: z.enum(["uncommitted", "unpushed", "unknown"]).catch("unknown"),
      count: z.number().catch(0),
    }).loose().catch({ branch: "", reason: "unknown", count: 0 }),
  }).loose(),
  z.object({
    ...WorklaneActivityShape,
    action: z.unknown().transform(() => "unknown" as const),
    details: z.record(z.string(), z.unknown()).catch({}),
  }).loose(),
]);

export const WorklaneActivityResponseSchema = z.object({
  entries: z.array(WorklaneActivityEntrySchema.catch(UNKNOWN_WORKLANE_ACTIVITY_ENTRY)).default([]),
}).loose();

export const EMPTY_WORKLANE_ACTIVITY_RESPONSE: WorklaneActivityResponse = {
  entries: [],
};

export const ReleaseSchema: z.ZodType<Release> = z.object({
  id: z.string(),
  repository_id: z.string(),
  position: z.number().default(0),
  version: z.string(),
  bump: z.enum(["patch", "minor", "major"]).nullable().catch(null),
  title: z.string(),
  description: z.string().default(""),
  shipped_at: z.string().nullable().default(null),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_RELEASE: Release = {
  id: "",
  repository_id: "",
  position: 0,
  version: "",
  bump: null,
  title: "",
  description: "",
  shipped_at: null,
  created_at: "",
  updated_at: "",
};

const ReleaseListSchema = z.array(ReleaseSchema.catch(EMPTY_RELEASE))
  .transform((releases) => releases.filter((release) => release.id !== ""))
  .default([]);

export const ReleasesResponseSchema = z.object({ releases: ReleaseListSchema }).loose();
export const ReleaseLineResponseSchema = z.object({ releases: ReleaseListSchema }).loose();
export const EMPTY_RELEASES_RESPONSE: ListReleasesResponse = { releases: [] };
export const EMPTY_RELEASE_LINE_RESPONSE: ReleaseLineResponse = { releases: [] };

export const ReleaseRefusalSchema: z.ZodType<ReleaseRefusal> = z.object({
  code: z.enum([
    "title_required", "version_or_bump", "version_invalid", "does_not_release",
    "last_tag_unknown", "first_release", "last_release", "line_not_increasing", "unknown",
  ]).catch("unknown"),
  error: z.string().optional(),
  release_id: z.string().optional(),
  version: z.string().optional(),
  title: z.string().optional(),
  before: z.union([
    z.object({ version: z.string(), title: z.string() }).loose(),
    z.object({ last_tag: z.string() }).loose(),
  ]).optional().catch(undefined),
}).loose();

export const UNKNOWN_RELEASE_REFUSAL: ReleaseRefusal = { code: "unknown" };

export const RepositoryRefusalSchema: z.ZodType<RepositoryRefusal> = z.object({
  code: z.enum(["has_unshipped_releases", "has_worklanes", "unknown"]).catch("unknown"),
  error: z.string().optional(),
  releases: z.array(z.object({
    version: z.string().catch(""),
    title: z.string().catch(""),
  }).loose()).catch([]),
  worklanes: z.array(z.string()).optional().catch(undefined),
}).loose();

export const UNKNOWN_REPOSITORY_REFUSAL: RepositoryRefusal = { code: "unknown", releases: [] };

const IssueTemplateFieldSchema = z.object({
  key: z.string(),
  label: z.string().default(""),
  required: z.boolean().default(false),
  placeholder: z.string().default(""),
  rows: z.number().default(1),
  format: z.enum(["text", "bullets", "checks", "scenarios"]).catch("text"),
}).loose();

const IssueTemplateSchema: z.ZodType<IssueTemplate> = z.object({
  // Unknown types invalidate only this template, never the whole catalogue.
  type: z.enum(ISSUE_TYPES),
  sections: z.array(z.object({
    heading: z.string().default(""),
    question: z.string().default(""),
    field: IssueTemplateFieldSchema.nullable().optional(),
  }).loose()).default([]),
}).loose();

export const IssueTemplatesResponseSchema = z.object({
  templates: z.array(IssueTemplateSchema.nullable().catch(null))
    .transform((templates) => templates.filter((template): template is IssueTemplate => template !== null))
    .default([]),
}).loose();

export const EMPTY_ISSUE_TEMPLATES_RESPONSE: IssueTemplatesResponse = { templates: [] };

export const WorklaneIssuesResponseSchema = z.object({
  issues: z.array(IssueSchema).default([]),
}).loose();

export const EMPTY_WORKLANE_ISSUES_RESPONSE: WorklaneIssuesResponse = { issues: [] };

const ReleaseProgressRowSchema = z.object({
  worklane_id: z.string(),
  release_id: z.string(),
  done: z.number().default(0),
  total: z.number().default(0),
}).loose();

export const ReleaseProgressResponseSchema = z.object({
  rows: z.array(ReleaseProgressRowSchema).default([]),
  releases: z.array(z.object({
    release_id: z.string(),
    done: z.number().default(0),
    total: z.number().default(0),
  }).loose()).default([]),
}).loose();

export const EMPTY_RELEASE_PROGRESS_RESPONSE: ReleaseProgressResponse = { rows: [], releases: [] };

export const IssuePlacementRefusalSchema: z.ZodType<IssuePlacementRefusal> = z.object({
  code: z.enum([
    "type_invalid", "type_fixed", "release_needs_worklane", "release_not_of_repository",
    "release_shipped", "issue_not_in_repository", "issue_in_release",
    "project_follows_worklane", "worklane_has_open_issues", "unknown",
  ]).catch("unknown"),
  error: z.string().optional(),
  version: z.string().optional(),
  title: z.string().optional(),
  repository: z.string().optional(),
  issues: z.array(z.object({
    id: z.string(),
    identifier: z.string(),
    title: z.string().optional(),
  }).loose()).default([]),
}).loose();

export const UNKNOWN_ISSUE_PLACEMENT_REFUSAL: IssuePlacementRefusal = { code: "unknown", issues: [] };

// Local search index sync (MUL-7754). Callers reject a response that fails
// these schemas instead of degrading to an empty value: an empty snapshot page
// or change set would be applied to the local copy as truth.
const SearchIndexIssueSchema = IssueSchema.extend({
  search_updated_at: z.string(),
}).loose();

const SearchIndexProjectSchema = ProjectSchema.extend({
  search_updated_at: z.string(),
}).loose();

const SearchIndexCommentSchema = z.object({
  id: z.string().min(1),
  issue_id: z.string().min(1),
  content: z.string(),
  created_at: z.string(),
}).loose();

export const SearchIndexManifestSchema = z.object({
  cursor: z.string().min(1),
  issue_count: z.number(),
  comment_count: z.number(),
  project_count: z.number(),
  text_bytes: z.number(),
}).loose();

export const SearchIndexSnapshotPageSchema = z.object({
  issues: z.array(SearchIndexIssueSchema),
  comments: z.array(SearchIndexCommentSchema),
  projects: z.array(SearchIndexProjectSchema),
  next_after_number: z.number(),
  done: z.boolean(),
}).loose();

export const SearchIndexChangesSchema = z.object({
  issues: z.array(SearchIndexIssueSchema),
  comments: z.array(SearchIndexCommentSchema),
  projects: z.array(SearchIndexProjectSchema),
  deleted: z.object({
    issues: z.array(z.string()),
    comments: z.array(z.string()),
    projects: z.array(z.string()),
  }).loose(),
  cursor: z.string().min(1),
  has_more: z.boolean(),
}).loose();

const IssueAssigneeGroupSchema = z.object({
  id: z.string(),
  assignee_type: z.string().nullable(),
  assignee_id: z.string().nullable(),
  issues: z.array(IssueSchema).default([]),
  total: z.number().default(0),
}).loose();

export const GroupedIssuesResponseSchema = z.object({
  groups: z.array(IssueAssigneeGroupSchema).default([]),
}).loose();

export const EMPTY_GROUPED_ISSUES_RESPONSE: GroupedIssuesResponse = {
  groups: [],
};

const IssueTableActorRefSchema = z.object({
  // Server-driven enums stay open so installed desktop clients survive a
  // backend that introduces another actor kind.
  type: z.string(),
  id: z.string(),
}).loose();

const IssueTableParentRefSchema = z.object({
  id: z.string(),
  number: z.number(),
  identifier: z.string(),
  title: z.string(),
  status: z.string(),
}).loose();

const IssueTableGroupValueSchema = z.discriminatedUnion("kind", [
  z.object({
    kind: z.literal("status"),
    // Preserve the requested bucket vocabulary (legacy wire, lifecycle, or
    // concrete status). Normalizing here would disconnect values from keys.
    status: z.string(),
  }).loose(),
  z.object({
    kind: z.literal("assignee"),
    actor: IssueTableActorRefSchema.nullable(),
  }).loose(),
  z.object({
    kind: z.literal("project"),
    project_id: z.string().nullable().optional().default(null),
  }).loose(),
  z.object({
    kind: z.literal("parent"),
    parent_id: z.string().nullable().optional().default(null),
    parent: IssueTableParentRefSchema.nullable().optional().default(null),
    value_state: z.enum(["value", "unavailable", "unset"]),
  }).loose(),
  z.object({
    kind: z.literal("property"),
    property_id: z.string(),
    value: z.union([z.string(), z.boolean(), z.null()]).optional(),
    value_state: z.enum(["value", "unavailable", "unset"]),
  }).loose(),
]);

const IssueTableGroupDescriptorSchema: z.ZodType<IssueTableGroupDescriptor> = z.lazy(() => z.object({
  key: z.string(),
  value: IssueTableGroupValueSchema,
  count: z.number(),
  secondary_groups: z.array(IssueTableGroupDescriptorSchema).optional(),
}).loose());

export const IssueTableGroupsResponseSchema = z.object({
  query_fingerprint: z.string(),
  total: z.number(),
  groups: z.array(IssueTableGroupDescriptorSchema).default([]),
  next_cursor: z.string().nullable().default(null),
}).loose();

export const EMPTY_ISSUE_TABLE_GROUPS_RESPONSE: IssueTableGroupsResponse = {
  query_fingerprint: "",
  total: 0,
  groups: [],
  next_cursor: null,
};

const IssueTableRowSchema = z.object({
  issue: IssueSchema,
  direct_child_count: z.number().default(0),
}).loose();

export const IssueTableRowsResponseSchema = z.object({
  query_fingerprint: z.string(),
  group_key: z.string().nullable().default(null),
  parent_id: z.string().nullable().default(null),
  total: z.number(),
  rows: z.array(IssueTableRowSchema).default([]),
  branch_total: z.number(),
  next_cursor: z.string().nullable().default(null),
}).loose();

export const EMPTY_ISSUE_TABLE_ROWS_RESPONSE: IssueTableRowsResponse = {
  query_fingerprint: "",
  group_key: null,
  parent_id: null,
  total: 0,
  rows: [],
  branch_total: 0,
  next_cursor: null,
};

const IssueTableFacetValueSchema = z.object({
  key: z.string(),
  count: z.number(),
}).loose();

const IssueTableFacetSchema = z.object({
  kind: z.enum(["status", "priority", "assignee", "creator", "project", "label", "property", "working_agents"]),
  property_id: z.string().optional(),
  values: z.array(IssueTableFacetValueSchema).default([]),
}).loose();

export const IssueTableFacetsResponseSchema = z.object({
  query_fingerprint: z.string(),
  total: z.number(),
  facets: z.array(IssueTableFacetSchema).default([]),
}).loose();

export const EMPTY_ISSUE_TABLE_FACETS_RESPONSE: IssueTableFacetsResponse = {
  query_fingerprint: "",
  total: 0,
  facets: [],
};

const SubscriberSchema = z.object({
  issue_id: z.string(),
  user_type: z.string(),
  user_id: z.string(),
  reason: z.string(),
  created_at: z.string(),
}).loose();

export const SubscribersListSchema = z.array(SubscriberSchema);

export const ChildIssuesResponseSchema = z.object({
  issues: z.array(IssueSchema).default([]),
}).loose();

export const IssueDuplicatesResponseSchema = z.object({
  duplicate_of: IssueSchema.nullable().default(null),
  duplicates: z.array(IssueSchema).default([]),
}).loose();

export const ChildIssueProgressResponseSchema = z.object({
  progress: z
    .array(
      z
        .object({
          parent_issue_id: z.string(),
          total: z.number(),
          done: z.number(),
        })
        .loose(),
    )
    .default([]),
}).loose();

export const CloudRuntimeNodeSchema = z.object({
  id: z.string(),
  owner_id: z.string(),
  instance_id: z.string(),
  region: z.string(),
  instance_type: z.string(),
  image_id: z.string(),
  subnet_id: z.string(),
  name: z.string(),
  status: z.string(),
  tags: z.record(z.string(), z.string()).default({}),
  metadata: z.record(z.string(), z.unknown()).default({}),
  created_at: z.string(),
  updated_at: z.string(),
}).loose();

export const CloudRuntimeNodeListSchema = z.array(CloudRuntimeNodeSchema);

export const EMPTY_CLOUD_RUNTIME_NODE_LIST: CloudRuntimeNode[] = [];

export const EMPTY_CLOUD_RUNTIME_NODE: CloudRuntimeNode = {
  id: "",
  owner_id: "",
  instance_id: "",
  region: "",
  instance_type: "",
  image_id: "",
  subnet_id: "",
  name: "",
  status: "",
  tags: {},
  metadata: {},
  created_at: "",
  updated_at: "",
};

// ---------------------------------------------------------------------------
// Workspace dashboard schemas
//
// The dashboard hits three independent rollup endpoints. Each returns a flat
// array, and every field is consumed by chart / KPI math — a missing number
// silently degrades to NaN downstream, so we coerce missing numbers to 0.
// String fields default to "" (no enum narrowing) to survive future model /
// agent ID drift, and so a single null from tz-aware SQL bucketing fails
// only that row instead of dropping the whole array to the `[]` fallback.
// ---------------------------------------------------------------------------

// Cost split carried by every usage row. `cost_usd_ticks` is what the provider
// itself charged for the rows behind this aggregate (1e-10 USD); the
// `uncosted_*` counts are the tokens from rows the provider did NOT price, and
// so are the only ones the client should run through its rate table.
//
// The `uncosted_*` fields are deliberately `.optional()` rather than
// `.default(0)`: a backend that predates them sends nothing, and defaulting
// those rows to "0 tokens left to estimate" would silently zero their cost.
// `undefined` means "this backend doesn't split", and the consumer falls back
// to the full token counts — i.e. exactly the old behaviour. A real 0 from a
// current backend means "everything here is already priced", which is a
// different thing and must stay distinguishable.
const CostSplitShape = {
  cost_usd_ticks: z.number().optional(),
  uncosted_input_tokens: z.number().optional(),
  uncosted_output_tokens: z.number().optional(),
  uncosted_cache_read_tokens: z.number().optional(),
  uncosted_cache_write_tokens: z.number().optional(),
};

const DashboardUsageDailySchema = z.object({
  date: z.string().default(""),
  provider: z.string().default(""),
  model: z.string().default(""),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  ...CostSplitShape,
  task_count: z.number().default(0),
}).loose();

export const DashboardUsageDailyListSchema = z.array(DashboardUsageDailySchema);

const DashboardUsageByAgentSchema = z.object({
  agent_id: z.string().default(""),
  provider: z.string().default(""),
  model: z.string().default(""),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  ...CostSplitShape,
  task_count: z.number().default(0),
}).loose();

export const DashboardUsageByAgentListSchema = z.array(DashboardUsageByAgentSchema);

// `cancelled_count` defaults to 0 so an installed client pointed at a
// backend that predates it still renders: those rows simply carry no
// cancelled segment, which is exactly what that backend measured.
const DashboardAgentRunTimeSchema = z.object({
  agent_id: z.string().default(""),
  total_seconds: z.number().default(0),
  task_count: z.number().default(0),
  metered_task_count: z.number().optional().catch(undefined),
  failed_count: z.number().default(0),
  cancelled_count: z.number().default(0),
}).loose();

export const DashboardAgentRunTimeListSchema = z.array(DashboardAgentRunTimeSchema);

const DashboardRunTimeDailySchema = z.object({
  date: z.string().default(""),
  total_seconds: z.number().default(0),
  task_count: z.number().default(0),
  failed_count: z.number().default(0),
  cancelled_count: z.number().default(0),
}).loose();

export const DashboardRunTimeDailyListSchema = z.array(DashboardRunTimeDailySchema);

// Failure rollups. `failure_reason` is an open string on purpose — it carries
// the backend's canonical taxonomy, which grows as new classifier rules land
// (server/pkg/taskfailure). Pinning it to a z.enum would make an installed
// desktop client drop rows for a reason its build predates; the client folds
// unrecognised reasons into an "other" display class instead. The empty
// string is the succeeded bucket, so `.default("")` is a meaningful default
// only for a row that already lost its reason — such a row lands in the
// denominator rather than inventing a failure that never happened.
const DashboardFailureDailySchema = z.object({
  date: z.string().default(""),
  failure_reason: z.string().default(""),
  task_count: z.number().default(0),
}).loose();

export const DashboardFailureDailyListSchema = z.array(DashboardFailureDailySchema);

const DashboardFailureByAgentSchema = z.object({
  agent_id: z.string().default(""),
  failure_reason: z.string().default(""),
  task_count: z.number().default(0),
}).loose();

export const DashboardFailureByAgentListSchema = z.array(
  DashboardFailureByAgentSchema,
);

// ---------------------------------------------------------------------------
// Runtime usage schemas — the runtime-detail page's four usage endpoints
// (`/api/runtimes/:id/usage*`). Same leniency rules as the dashboard
// schemas above: numbers default to 0, strings to "", `.loose()` passes
// unknown fields.
// ---------------------------------------------------------------------------

const RuntimeUsageSchema = z.object({
  runtime_id: z.string().default(""),
  date: z.string().default(""),
  provider: z.string().default(""),
  model: z.string().default(""),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  ...CostSplitShape,
}).loose();

export const RuntimeUsageListSchema = z.array(RuntimeUsageSchema);

const RuntimeHourlyActivitySchema = z.object({
  hour: z.number().default(0),
  count: z.number().default(0),
}).loose();

export const RuntimeHourlyActivityListSchema = z.array(RuntimeHourlyActivitySchema);

const RuntimeUsageByAgentSchema = z.object({
  agent_id: z.string().default(""),
  provider: z.string().default(""),
  model: z.string().default(""),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  ...CostSplitShape,
  task_count: z.number().default(0),
}).loose();

export const RuntimeUsageByAgentListSchema = z.array(RuntimeUsageByAgentSchema);

const RuntimeUsageByHourSchema = z.object({
  hour: z.number().default(0),
  model: z.string().default(""),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  ...CostSplitShape,
  task_count: z.number().default(0),
}).loose();

export const RuntimeUsageByHourListSchema = z.array(RuntimeUsageByHourSchema);

// ---------------------------------------------------------------------------
// Agent task responses. The base object stays loose so daemon/runtime fields
// can drift while task-list consumers still validate the fields they render.
// ---------------------------------------------------------------------------

// Human attribution (MUL-4302 §9): who an agent run is accountable to, and how
// that human was resolved. Every field is defensive so a departed member, an
// autopilot run (no originator), or an older backend degrades to a partial
// object instead of a parse failure.
const AttributionUserSchema = z.object({
  id: z.string().default(""),
  name: z.string().optional(),
  email: z.string().optional(),
  avatar_url: z.string().optional(),
}).loose();

const TaskEvidenceSchema = z.object({
  kind: z.string().default(""),
  ref_id: z.string().default(""),
}).loose();

const TaskAttributionSchema = z.object({
  source: z.string().default("unattributed"),
  precise: z.boolean().default(false),
  initiator: AttributionUserSchema.optional(),
  originator: AttributionUserSchema.optional(),
  evidence: TaskEvidenceSchema.optional(),
  rule_version_id: z.string().optional(),
  delegated_from_task_id: z.string().optional(),
  retry_of_task_id: z.string().optional(),
  rerun_of_task_id: z.string().optional(),
}).loose();

const TaskCancellationActorSchema = z.object({
  type: z.string().default(""),
  id: z.string().optional(),
  name: z.string().optional(),
}).loose();

const OptionalStringArraySchema = z.preprocess(
  (value) =>
    Array.isArray(value) && value.every((item) => typeof item === "string")
      ? value
      : undefined,
  z.array(z.string()).optional(),
);

// One (provider, model) slice of a run's token usage. Token counts default to
// 0 rather than failing the row: a slice missing one counter is still worth
// pricing on the counters it does have, and the "we have no usage at all" case
// is carried by the field's absence, not by a zeroed entry.
const TaskUsageSchema = z.object({
  provider: z.string().optional(),
  model: z.string().default(""),
  input_tokens: z.number().default(0),
  output_tokens: z.number().default(0),
  cache_read_tokens: z.number().default(0),
  cache_write_tokens: z.number().default(0),
  cost_usd_ticks: z.number().optional(),
}).loose();

export const AgentTaskSchema = z.object({
  wakeup_id: z.string().optional().catch(undefined),
  cancelled_by_comment_change: z.boolean().optional().catch(undefined),
  cancelled_by: TaskCancellationActorSchema.optional().catch(undefined),
  id: z.string(),
  agent_id: z.string().default(""),
  runtime_id: z.string().default(""),
  issue_id: z.string().default(""),
  status: z.string().default("cancelled"),
  priority: z.number().default(0),
  dispatched_at: z.string().nullable().default(null),
  started_at: z.string().nullable().default(null),
  completed_at: z.string().nullable().default(null),
  result: z.unknown().default(null),
  error: z.string().nullable().default(null),
  failure_reason: z.string().optional(),
  created_at: z.string().default(""),
  chat_session_id: z.string().optional(),
  autopilot_run_id: z.string().optional(),
  parent_task_id: z.string().optional(),
  attempt: z.number().optional(),
  trigger_comment_id: z.string().optional(),
  // Coverage is additive display metadata. A mixed-version or partially
  // upgraded server must not make one malformed optional field erase the
  // entire execution log, so degrade that field to "absent" independently.
  coalesced_comment_ids: OptionalStringArraySchema,
  delivered_comment_ids: OptionalStringArraySchema,
  supplement_capability: z.string().optional().catch(undefined),
  supplement_comment_ids: OptionalStringArraySchema,
  can_supplement: z.boolean().optional().catch(undefined),
  trigger_summary: z.string().optional(),
  kind: z.string().optional(),
  work_dir: z.string().optional().catch(undefined),
  relative_work_dir: z.string().optional().catch(undefined),
  durable_work_dir: z.string().optional().catch(undefined),
  relative_durable_work_dir: z.string().optional().catch(undefined),
  branch_name: z.string().optional().catch(undefined),
  attribution: TaskAttributionSchema.optional(),
  // Per-run token usage. Same independent-degradation rule as the coverage
  // arrays above: usage is additive display metadata, so one malformed entry
  // must cost the row its usage figure, not erase the whole execution log.
  // `.catch(undefined)` collapses a bad array to "no usage recorded", which
  // the UI already renders as an em dash.
  usage: z.array(TaskUsageSchema).optional().catch(undefined),
}).loose();

// Outcome counts are required: every backend that serves this endpoint
// reports them, so a response missing them is a contract drift, not an
// older peer. `parseWithFallback` then degrades the whole list to `[]`
// rather than letting a partial window masquerade as measured outcomes.
export const AgentActivityBucketListSchema = z.array(z.object({
  agent_id: z.string(),
  bucket_at: z.string(),
  task_count: z.number().int().nonnegative(),
  failed_count: z.number().int().nonnegative(),
  completed_count: z.number().int().nonnegative(),
  cancelled_count: z.number().int().nonnegative(),
  duration_ms: z.number().nonnegative().optional().catch(undefined),
  duration_count: z.number().int().nonnegative().optional().catch(undefined),
}).loose());

export const AgentTaskListSchema = z.array(AgentTaskSchema);

export const AgentTaskPageSchema = z.object({
  tasks: AgentTaskListSchema,
  nextCursor: z.string().min(1).nullable(),
});

// One row of a run transcript. `output_truncated` gates a completeness claim
// the UI makes about a tool's output, so it stays `.optional()` with no
// default: a server that does not send it means "unknown", and defaulting it
// to false would turn a missing field into an assertion that the record is
// complete. `.catch(undefined)` keeps a malformed value that far too — without
// it a single bad boolean fails its row, the array fails with it, and
// `parseWithFallback` hands the viewer an empty transcript. Degrading one
// field to "unknown" is the correct loss; deleting the run is not. Every other
// field keeps a default for the same reason.
export const TaskMessagePayloadSchema = z.object({
  call_id: z.string().optional().catch(undefined),
  task_id: z.string().default(""),
  issue_id: z.string().default(""),
  chat_session_id: z.string().optional(),
  seq: z.number().default(0),
  type: z.enum(["text", "thinking", "tool_use", "tool_result", "error"]).catch("text"),
  tool: z.string().optional(),
  content: z.string().optional(),
  input: z.record(z.string(), z.unknown()).optional(),
  output: z.string().optional(),
  output_truncated: z.boolean().optional().catch(undefined),
  created_at: z.string().optional(),
}).loose();

export const TaskMessageListSchema = z.array(TaskMessagePayloadSchema).default([]);

// Task cancellation (`POST /api/tasks/:id/cancel`) is consumed directly by
// chat recovery. Its optional message payload must be well-formed before the
// UI deletes a message from cache or restores text into the input.
const CancelledChatMessageSchema = z.object({
  chat_session_id: z.string(),
  message_id: z.string(),
  content: z.string(),
  restore_to_input: z.boolean().default(false),
  // Attachments detached from the deleted message so a restored draft can
  // re-bind them on re-send. Absent on servers that predate the field.
  attachments: z.array(AttachmentSchema).optional(),
}).loose();

export const CancelTaskResponseSchema = AgentTaskSchema.extend({
  cancelled_chat_message: CancelledChatMessageSchema.nullish()
    .transform((value) => value ?? undefined),
}).loose();

const ChatLastMessageSchema = z.object({
  content: z.string().default(""),
  role: z.enum(["user", "assistant"]).catch("assistant"),
  created_at: z.string().default(""),
  failure_reason: z.string().nullable().optional(),
  message_kind: z.enum([
    "message",
    "no_response",
    "onboarding_kickoff",
    "onboarding_opening",
  ]).optional().catch(undefined),
}).loose();

const ChatChannelSourceSchema = z.object({
  channel_type: z.string().default(""),
  installation_id: z.string().default(""),
  route_revision: z.number().default(0),
}).loose();

export const ChatSessionSchema: z.ZodType<ChatSession> = z.object({
  id: z.string(),
  workspace_id: z.string().default(""),
  agent_id: z.string().default(""),
  creator_id: z.string().default(""),
  project_id: z.string().nullable().optional(),
  title: z.string().default(""),
  status: z.enum(["active", "archived"]).catch("active"),
  has_unread: z.boolean().default(false),
  unread_count: z.number().optional(),
  last_message: ChatLastMessageSchema.nullable().optional().catch(undefined),
  pinned: z.boolean().optional(),
  channel_source: ChatChannelSourceSchema.optional().catch(undefined),
  is_current_channel_route: z.boolean().optional().catch(undefined),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_CHAT_SESSION: ChatSession = {
  id: "",
  workspace_id: "",
  agent_id: "",
  creator_id: "",
  title: "",
  status: "active",
  has_unread: false,
  created_at: "",
  updated_at: "",
};
export const ChatSessionListSchema = z
  .array(ChatSessionSchema.catch(EMPTY_CHAT_SESSION))
  .transform((sessions) => sessions.filter((session) => session.id !== ""))
  .default([]);
export const EMPTY_CHAT_SESSION_LIST: ChatSession[] = [];

// Deferred-cancellation draft restores
// (`GET /api/chat/sessions/{id}/draft-restores`, #5219) feed the composer
// directly: `content` becomes the draft text, `attachments` re-bind on
// re-send, and `id` is the consume key. A malformed response falls back to
// an empty list — the durable row stays pending server-side, so nothing is
// lost by skipping a fetch.
const ChatDraftRestoreSchema = z.object({
  id: z.string(),
  chat_session_id: z.string(),
  task_id: z.string().optional(),
  content: z.string().default(""),
  attachments: z.array(AttachmentSchema).optional(),
  created_at: z.string().optional(),
}).loose();

export const ChatDraftRestoresResponseSchema = z.object({
  restores: z.array(ChatDraftRestoreSchema).default([]),
}).loose();

const ChatQueuedTaskSchema = z.object({
  task_id: z.string(),
  status: z.string().default("queued"),
  created_at: z.string().default(""),
  message_id: z.string().optional(),
  content: z.string().optional(),
}).loose();

const ChatQueuedTasksSchema = z.array(z.unknown()).transform((tasks) =>
  tasks.flatMap((task) => {
    const parsed = ChatQueuedTaskSchema.safeParse(task);
    return parsed.success ? [parsed.data] : [];
  }),
);

// Root fields retain the legacy single-task response shape. Keep additive
// fields optional so callers can distinguish an older server from an empty
// queue. A malformed queue row is ignored without discarding a valid head.
export const ChatPendingTaskSchema: z.ZodType<ChatPendingTask> = z.object({
  task_id: z.string().optional(),
  status: z.string().optional(),
  created_at: z.string().optional(),
  supports_queue: z.boolean().optional(),
  queued_tasks: ChatQueuedTasksSchema.optional(),
}).loose();

export const EMPTY_CHAT_PENDING_TASK: ChatPendingTask = {};

export const SendChatMessageResponseSchema: z.ZodType<SendChatMessageResponse> = z.object({
  message_id: z.string().min(1),
  task_id: z.string().min(1),
  supports_queue: z.boolean().optional(),
  queued: z.boolean().optional().catch(undefined),
  created_at: z.string().min(1),
  attachment_ids: z.array(z.string()).nullish().transform((ids) => ids ?? undefined),
}).loose();

// `started` is the only field the flow branches on, and a malformed response
// must not be read as "the opening landed" — parseWithFallback's fallback says
// it did not, which leaves the flow's own retry as the recovery path.
export const StartMikaOnboardingResponseSchema: z.ZodType<StartMikaOnboardingResponse> = z.object({
  started: z.boolean(),
  message_id: z.string().nullish().transform((id) => id ?? undefined),
  created_at: z.string().nullish().transform((at) => at ?? undefined),
}).loose();

export const PrioritizeQueuedChatTaskResponseSchema:
  z.ZodType<PrioritizeQueuedChatTaskResponse> = z.object({
    task_id: z.string(),
    active_task_id: z.string().optional(),
  }).loose();

export const EMPTY_PRIORITIZE_QUEUED_CHAT_TASK_RESPONSE:
  PrioritizeQueuedChatTaskResponse = { task_id: "" };

export const EMPTY_CHAT_DRAFT_RESTORES: ChatDraftRestoresResponse = {
  restores: [],
};

export const EMPTY_CANCEL_TASK_RESPONSE: CancelTaskResponse = {
  id: "",
  agent_id: "",
  runtime_id: "",
  issue_id: "",
  status: "cancelled",
  priority: 0,
  dispatched_at: null,
  started_at: null,
  completed_at: null,
  result: null,
  error: null,
  created_at: "",
};

export const AgentBuilderSessionSchema = z.object({
  session_id: z.string(),
  builder_agent_id: z.string(),
  runtime_id: z.string(),
}).loose();

export const EMPTY_AGENT_BUILDER_SESSION: AgentBuilderSession = {
  session_id: "",
  builder_agent_id: "",
  runtime_id: "",
};

/**
 * The stored configuration of a creation conversation. Every field falls back
 * to empty on its own: a draft written by a newer build (or truncated in
 * transit) must still restore the fields it does understand rather than
 * discarding the user's work wholesale.
 */
export const StoredAgentDraftSchema = z.object({
  name: z.string().catch(""),
  description: z.string().catch(""),
  instructions: z.string().catch(""),
  instruction_selection: z.array(z.string()).catch([]),
  conversation_starters: z
    .array(
      z.object({
        label: z.string().catch(""),
        prompt: z.string().catch(""),
      }),
    )
    .catch([]),
  avatar_url: z.string().nullable().catch(null),
  model: z.string().catch(""),
  thinking_level: z.string().catch(""),
  service_tier: z.string().catch(""),
  skill_ids: z.array(z.string()).catch([]),
  permission_scope: z
    .enum(["private", "workspace", "members"])
    .catch("private"),
  member_ids: z.array(z.string()).catch([]),
  team_ids: z.array(z.string()).catch([]),
  applied_message_id: z.string().nullable().catch(null),
}).loose();

/**
 * One unfinished creation draft. Every field except the id has a safe empty
 * default: an older server that omits `runtime_id` must degrade to "let the
 * user pick" rather than dropping the whole row and losing the conversation.
 */
export const AgentBuilderSessionSummarySchema = z.object({
  session_id: z.string(),
  title: z.string().catch(""),
  runtime_id: z.string().catch(""),
  created_at: z.string().catch(""),
  updated_at: z.string().catch(""),
  last_message_content: z.string().catch(""),
  last_message_role: z.string().catch(""),
  last_message_at: z.string().catch(""),
  // Absent for a conversation the user has never hand-edited; the client then
  // replays the last <agent_draft> block instead of restoring a stored copy.
  draft: StoredAgentDraftSchema.nullish().catch(null),
}).loose();

export const AgentBuilderSessionListSchema = z.object({
  sessions: z.array(AgentBuilderSessionSummarySchema).catch([]),
}).loose();

export const EMPTY_AGENT_BUILDER_SESSION_LIST: {
  sessions: AgentBuilderSessionSummary[];
} = { sessions: [] };

export const AgentBuilderRuntimeSwitchSchema = z.object({
  runtime_id: z.string(),
}).loose();

// This endpoint returns 2xx only after the carrier has been bound to the
// runtime the caller asked for; anything else is a thrown error and no commit.
// So the safe fallback for an unparseable SUCCESS body is the requested id, not
// an empty one: the rebind did happen, and reporting "unknown" would leave the
// picker showing a runtime that is no longer executing — the exact split this
// endpoint exists to close.
export const agentBuilderRuntimeSwitchFallback = (
  requestedRuntimeID: string,
): AgentBuilderRuntimeSwitch => ({ runtime_id: requestedRuntimeID });

// Squad list responses carry lightweight membership previews used by hover
// cards. The preview fields are additive API fields, so older backends default
// cleanly to no preview instead of breaking newer frontends.
const SquadMemberPreviewSchema = z.object({
  member_type: z.string(),
  member_id: z.string(),
  role: z.string().default(""),
}).loose();

export const SquadSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  name: z.string(),
  description: z.string().default(""),
  instructions: z.string().default(""),
  system_key: z.string().optional(),
  system_instructions: z.string().optional(),
  avatar_url: z.string().nullable().optional().transform((v) => v ?? null),
  leader_id: z.string(),
  creator_id: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
  archived_at: z.string().nullable().optional().transform((v) => v ?? null),
  archived_by: z.string().nullable().optional().transform((v) => v ?? null),
  member_count: z.number().default(0),
  member_preview: z.array(SquadMemberPreviewSchema).default([]),
}).loose();

export const SquadListSchema = z.array(SquadSchema);
export const EMPTY_SQUAD_LIST: Squad[] = [];
export const EMPTY_SQUAD: Squad = {
  id: "",
  workspace_id: "",
  name: "",
  description: "",
  instructions: "",
  avatar_url: null,
  leader_id: "",
  creator_id: "",
  created_at: "",
  updated_at: "",
  archived_at: null,
  archived_by: null,
  member_count: 0,
  member_preview: [],
};

// Squad member status — backs the Squad detail page's Members tab. status
// is `string | null` (not the narrow `SquadMemberStatusValue` union) so a
// new server-side status doesn't fail the parse; the UI defaults to a
// neutral pill for unknown values.
const SquadActiveIssueBriefSchema = z.object({
  issue_id: z.string(),
  identifier: z.string(),
  title: z.string(),
  issue_status: z.string(),
}).loose();

const SquadMemberStatusSchema = z.object({
  member_type: z.string(),
  member_id: z.string(),
  status: z.string().nullable().optional().transform((v) => v ?? null),
  active_issues: z.array(SquadActiveIssueBriefSchema).default([]),
  last_active_at: z.string().nullable().optional().transform((v) => v ?? null),
}).loose();

export const SquadMemberStatusListResponseSchema = z.object({
  members: z.array(SquadMemberStatusSchema).default([]),
}).loose();

export const EMPTY_SQUAD_MEMBER_STATUS_LIST = { members: [] };

// ---------------------------------------------------------------------------
// Structured error body — POST /api/workspaces/:wsId/issues 409 conflict.
//
// When the server detects an active issue with the same title in the same
// workspace, it returns `{ code: "active_duplicate_issue", error, issue }`
// instead of letting the create through. The UI uses the embedded issue ref
// to offer "view existing" rather than dropping the user into a generic
// "create failed" toast.
//
// Strict guarantees:
//   - `code` is a literal so a future server rename (e.g. `duplicate_issue`)
//     fails the parse and falls back to a normal error toast — drift never
//     ships as a broken duplicate UI.
//   - `issue` is required; without an id/identifier/title the "view existing"
//     button has nothing to point at, so we'd rather fall back than guess.
//   - `issue.status` is intentionally OMITTED: the duplicate toast doesn't
//     render a StatusIcon (which has no fallback for unknown enum values),
//     so a future server-side rename of `status` must not knock this branch
//     out. `.loose()` lets the field pass through unchanged for any other
//     consumer.
// ---------------------------------------------------------------------------

export const DuplicateIssueErrorBodySchema = z.object({
  code: z.literal("active_duplicate_issue"),
  error: z.string().optional(),
  issue: z.object({
    id: z.string(),
    identifier: z.string(),
    title: z.string(),
  }).loose(),
}).loose();

export interface DuplicateIssueErrorBody {
  code: "active_duplicate_issue";
  error?: string;
  issue: {
    id: string;
    identifier: string;
    title: string;
  };
}

// ---------------------------------------------------------------------------
// Webhook delivery schemas — backing the Autopilot Deliveries section. Enums
// (`status`, `signature_status`, `provider`) are kept as `z.string()` so a
// future server-side value (e.g. a Stripe provider, a new dedupe state)
// degrades to a generic UI fallback rather than collapsing the list into
// the empty array. `.loose()` lets unknown fields pass through, matching
// the rule used by every other endpoint here.
// ---------------------------------------------------------------------------

const WebhookDeliverySchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  autopilot_id: z.string(),
  trigger_id: z.string(),
  provider: z.string(),
  event: z.string(),
  dedupe_key: z.string().nullable(),
  dedupe_source: z.string().nullable(),
  signature_status: z.string(),
  status: z.string(),
  attempt_count: z.number().default(0),
  // Older servers predate the durable dispatch queue. Defaults preserve
  // compatibility while the UI rolls out alongside the new worker.
  dispatch_attempts: z.number().default(0),
  available_at: z.string().default(""),
  content_type: z.string().nullable(),
  response_status: z.number().nullable(),
  autopilot_run_id: z.string().nullable(),
  replayed_from_delivery_id: z.string().nullable(),
  error: z.string().nullable(),
  reason_code: z.string().nullable().default(null),
  replay_idempotency_key: z.string().nullable().default(null),
  received_at: z.string(),
  last_attempt_at: z.string(),
  created_at: z.string(),
  // Detail-only fields. The list endpoint omits them; the detail endpoint
  // populates raw_body / selected_headers / response_body.
  selected_headers: z.record(z.string(), z.unknown()).nullable().optional(),
  raw_body: z.string().nullable().optional(),
  response_body: z.string().nullable().optional(),
}).loose();

export const ListWebhookDeliveriesResponseSchema = z.object({
  deliveries: z.array(WebhookDeliverySchema).default([]),
  total: z.number().default(0),
}).loose();

export const WebhookDeliveryResponseSchema = WebhookDeliverySchema;

export const EMPTY_LIST_WEBHOOK_DELIVERIES_RESPONSE: ListWebhookDeliveriesResponse = {
  deliveries: [],
  total: 0,
};

// ---------------------------------------------------------------------------
// Autopilot list schema. Enums (`status`, `execution_mode`, `trigger_kinds`,
// `last_run_status`) stay `z.string()` so future server-side values degrade
// to a generic UI fallback. The three derived fields (trigger_kinds /
// next_run_at / last_run_status) are list-endpoint-only and absent on older
// servers — optional by contract, the list renders "—" without them.
// ---------------------------------------------------------------------------

const AutopilotListItemSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  title: z.string(),
  description: z.string().nullable().optional(),
  project_id: z.string().nullable().optional(),
  // Older servers (pre-MUL-2429) omit assignee_type; "agent" is the
  // documented default.
  assignee_type: z.string().default("agent"),
  assignee_id: z.string(),
  status: z.string(),
  execution_mode: z.string(),
  issue_title_template: z.string().nullable().optional(),
  created_by_type: z.string(),
  created_by_id: z.string(),
  last_run_at: z.string().nullable().optional(),
  created_at: z.string(),
  updated_at: z.string(),
  trigger_kinds: z.array(z.string()).optional(),
  next_run_at: z.string().nullable().optional(),
  last_run_status: z.string().nullable().optional(),
  // Per-caller write capability; absent on older servers (treated as unknown).
  can_write: z.boolean().optional(),
  // Narrower per-caller access-management capability (detail endpoint only).
  can_manage_access: z.boolean().optional(),
}).loose();

export const ListAutopilotsResponseSchema = z.object({
  autopilots: z.array(AutopilotListItemSchema).default([]),
  total: z.number().default(0),
}).loose();

export const EMPTY_LIST_AUTOPILOTS_RESPONSE = {
  autopilots: [],
  total: 0,
};

// Autopilot run (POST /trigger, GET /runs). Consumed by the "run now" flow,
// which branches on `status` to avoid a false-success toast (MUL-4525), so the
// response must be schema-parsed. `reason_code` is an additive, stable
// classification of a non-success run the UI localizes; older servers omit it.
// Defaults are conservative: an unreadable run degrades to a non-success status
// so the UI never shows success it cannot confirm. .loose() tolerates new fields.
export const AutopilotRunSchema = z.object({
  id: z.string().default(""),
  autopilot_id: z.string().default(""),
  trigger_id: z.string().nullable().default(null),
  source: z.string().default("manual"),
  status: z.string().default("failed"),
  issue_id: z.string().nullable().default(null),
  task_id: z.string().nullable().default(null),
  triggered_at: z.string().default(""),
  completed_at: z.string().nullable().default(null),
  failure_reason: z.string().nullable().default(null),
  reason_code: z.string().optional(),
  trigger_payload: z.unknown().default(null),
  result: z.unknown().default(null),
  created_at: z.string().default(""),
}).loose();

export const AutopilotQuotaUsageSchema = z.object({
  action: z.enum(["off", "observe", "enforce"]).default("off"),
  used: z.number().nullable().default(null),
  reserved: z.number().nullable().default(null),
  total: z.number().nullable().default(null),
  limit: z.number().nullable().default(null),
  reached: z.boolean().nullable().default(null),
  period_start: z.string().nullable().default(null),
  period_end: z.string().nullable().default(null),
  reset_at: z.string().nullable().default(null),
  blocked_counts: z.record(z.string(), z.number().int().nonnegative()).nullable().catch(null).default(null),
}).loose();

export const FALLBACK_AUTOPILOT_RUN: AutopilotRun = {
  id: "",
  autopilot_id: "",
  trigger_id: null,
  source: "manual",
  status: "failed",
  issue_id: null,
  task_id: null,
  triggered_at: "",
  completed_at: null,
  failure_reason: null,
  trigger_payload: null,
  result: null,
  created_at: "",
};

// Cron preview: the server is the authority on the next occurrences. No
// `.default([])` here — a missing or reshaped field must fail validation so it
// degrades to the `next_runs: null` fallback ("preview unreadable") instead of
// masquerading as a valid empty list ("this expression never fires").
export const CronPreviewResponseSchema = z.object({
  next_runs: z.array(z.string()),
}).loose();

export const UNREADABLE_CRON_PREVIEW_RESPONSE: CronPreviewResponse = {
  next_runs: null,
};

export const EMPTY_WEBHOOK_DELIVERY: WebhookDelivery = {
  id: "",
  workspace_id: "",
  autopilot_id: "",
  trigger_id: "",
  provider: "",
  event: "",
  dedupe_key: null,
  dedupe_source: null,
  signature_status: "not_required",
  status: "queued",
  attempt_count: 0,
  dispatch_attempts: 0,
  available_at: "",
  content_type: null,
  response_status: null,
  autopilot_run_id: null,
  replayed_from_delivery_id: null,
  error: null,
  reason_code: null,
  replay_idempotency_key: null,
  received_at: "",
  last_attempt_at: "",
  created_at: "",
};

// ---------------------------------------------------------------------------
// User (`/api/me` GET + PATCH). The auth store and Settings → Account both
// trust this shape — a drift here would knock both surfaces out. Kept
// lenient by the same rules as IssueSchema: enums stay `z.string()`,
// nullable fields are unioned with `null`, unknown server fields pass
// through via `.loose()`. `profile_description` is the field added in
// MUL-2406; the server emits `""` when unset (NOT NULL DEFAULT ''), so
// the schema defaults to `""` too — keeps the type tight without
// breaking older backends that don't return the column yet.
// ---------------------------------------------------------------------------

export const UserSchema = z.object({
  id: z.string(),
  name: z.string().default(""),
  email: z.string().default(""),
  avatar_url: z.string().nullable().default(null),
  onboarded_at: z.string().nullable().default(null),
  onboarding_questionnaire: z.record(z.string(), z.unknown()).default({}),
  starter_content_state: z.string().nullable().default(null),
  language: z.string().nullable().default(null),
  profile_description: z.string().default(""),
  timezone: z.string().nullable().default(null),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_USER: User = {
  id: "",
  name: "",
  email: "",
  avatar_url: null,
  onboarded_at: null,
  onboarding_questionnaire: {},
  starter_content_state: null,
  language: null,
  profile_description: "",
  timezone: null,
  created_at: "",
  updated_at: "",
};

// ---------------------------------------------------------------------------
// Cross-workspace unread inbox summary (`/api/inbox/unread-summary` GET).
// One entry per workspace the user belongs to that has unread items; the
// sidebar derives the workspace-switcher dot from it. Lenient per the usual
// rules so a future field addition can't blank the dot — on malformed JSON
// parseWithFallback returns the empty list, which simply hides the dot.
// ---------------------------------------------------------------------------

export const InboxUnreadSummarySchema = z.array(
  z
    .object({
      workspace_id: z.string(),
      count: z.number(),
    })
    .loose(),
);

export const EMPTY_INBOX_UNREAD_SUMMARY: InboxWorkspaceUnread[] = [];

// ---------------------------------------------------------------------------
// Inbox items (`/api/inbox` and `/api/inbox/archived` GET).
// Lenient per the usual rules: `severity` / `type` / `recipient_type` stay
// `z.string()` so a notification kind this client doesn't know yet still
// parses and renders (the UI's type-label lookup already tolerates unknown
// kinds). Nullable optional fields are declared optional as well, since older
// rows can omit them entirely. On malformed JSON parseWithFallback returns the
// empty list — the affected view then reads as empty rather than white-
// screening the inbox. Both endpoints share this boundary because they return
// the same row shape and both feed the status/priority filter UI.
// ---------------------------------------------------------------------------

export const InboxItemListSchema = z.array(
  z
    .object({
      id: z.string(),
      workspace_id: z.string(),
      recipient_type: z.string(),
      recipient_id: z.string(),
      type: z.string(),
      severity: z.string(),
      issue_id: z.string().nullish(),
      title: z.string(),
      body: z.string().nullish(),
      issue_status: z.string().nullish(),
      issue_priority: z.string().nullish(),
      read: z.boolean(),
      archived: z.boolean(),
      created_at: z.string(),
    })
    .loose(),
);

export const ArchivedInboxPageSchema = z.object({
  items: InboxItemListSchema,
  next_cursor: z.string().min(1).nullable(),
  has_more: z.boolean(),
}).refine((page) => page.has_more === (page.next_cursor !== null) &&
  (!page.has_more || page.items.length > 0))
  .transform((page) => ({ items: page.items, nextCursor: page.next_cursor, hasMore: page.has_more }));

export const ArchivedInboxFacetsSchema = z.object({
  statuses: z.record(z.string(), z.number().int().nonnegative()),
  priorities: z.record(z.string(), z.number().int().nonnegative()),
  actors: z.record(z.string(), z.number().int().nonnegative()),
  unread_count: z.number().int().nonnegative(),
}).transform((facets) => ({
  statuses: facets.statuses, priorities: facets.priorities,
  actors: facets.actors, unreadCount: facets.unread_count,
}));

export const EMPTY_INBOX_ITEMS: InboxItem[] = [];

// ---------------------------------------------------------------------------
// Billing schemas (cloud-billing proxy surface)
//
// All billing JSON we receive comes from multica-cloud verbatim — we proxy
// the bytes without re-shaping. These schemas use `loose()` so a future
// non-breaking field addition on the cloud side doesn't crash us; required
// fields are still strictly enforced. EMPTY_* constants supply the
// fallback parseWithFallback uses when the upstream response is malformed
// or unparseable.

export const BillingBalanceSchema = z.object({
  owner_id: z.string(),
  balance_micro: z.number(),
  balance_credit: z.number(),
  updated_at: z.string(),
}).loose();

export const EMPTY_BILLING_BALANCE: BillingBalance = {
  owner_id: "",
  balance_micro: 0,
  balance_credit: 0,
  updated_at: "",
};

// `tx_type` and `source` are kept as plain strings here; the cloud doc
// enumerates the canonical values but the frontend display tolerates
// unknown ones gracefully. Strict enums would crash the page on a future
// addition (e.g. a new `topup` source kind).
export const BillingTransactionSchema = z.object({
  id: z.string(),
  owner_id: z.string(),
  idempotency_key: z.string().default(""),
  tx_type: z.string(),
  source: z.string(),
  amount_micro: z.number(),
  balance_after: z.number(),
  reference_id: z.string().default(""),
  description: z.string().default(""),
  metadata: z.record(z.string(), z.unknown()).default({}),
  created_at: z.string(),
}).loose();

export const BillingTransactionsPageSchema = z.object({
  items: z.array(BillingTransactionSchema).default([]),
  total: z.number().default(0),
  page: z.number().default(1),
  page_size: z.number().default(20),
}).loose();

export const EMPTY_BILLING_TRANSACTIONS_PAGE: BillingTransactionsPage = {
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
};

export const BillingBatchSchema = z.object({
  id: z.string(),
  owner_id: z.string(),
  source_tx_id: z.string().default(""),
  source_type: z.string(),
  total_micro: z.number(),
  remaining_micro: z.number(),
  // Cloud either omits the key (never expires) or sends a string
  // timestamp. Null is also tolerated since some serializers emit
  // explicit nulls for absent timestamps.
  expires_at: z.string().nullable().optional(),
  created_at: z.string(),
  updated_at: z.string(),
}).loose();

export const BillingBatchesPageSchema = z.object({
  items: z.array(BillingBatchSchema).default([]),
  total: z.number().default(0),
  page: z.number().default(1),
  page_size: z.number().default(20),
}).loose();

export const EMPTY_BILLING_BATCHES_PAGE: BillingBatchesPage = {
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
};

export const BillingTopupSchema = z.object({
  id: z.string(),
  owner_id: z.string(),
  amount_cents: z.number(),
  currency: z.string().default("usd"),
  credits: z.number(),
  bonus_credits: z.number().default(0),
  status: z.string(),
  tier_id: z.string().default(""),
  stripe_checkout_id: z.string().default(""),
  // Only set after status reaches `credited` — leave optional rather
  // than coerce to "" so a UI can branch on existence.
  purchase_batch_id: z.string().optional(),
  created_at: z.string(),
  updated_at: z.string(),
}).loose();

export const BillingTopupsPageSchema = z.object({
  items: z.array(BillingTopupSchema).default([]),
  total: z.number().default(0),
  page: z.number().default(1),
  page_size: z.number().default(20),
}).loose();

export const EMPTY_BILLING_TOPUPS_PAGE: BillingTopupsPage = {
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
};

export const BillingPriceTierSchema = z.object({
  id: z.string(),
  // Cloud doc says display_name falls back to id; tolerate empty too.
  display_name: z.string().default(""),
  amount_cents: z.number(),
  credits: z.number(),
  bonus_credits: z.number().optional(),
  bonus_expires_in: z.string().optional(),
}).loose();

export const BillingPriceTierListSchema = z.array(BillingPriceTierSchema);

export const EMPTY_BILLING_PRICE_TIER_LIST: BillingPriceTier[] = [];

export const CreateBillingCheckoutSessionResponseSchema = z.object({
  order_id: z.string(),
  session_id: z.string(),
  url: z.string(),
}).loose();

export const EMPTY_CREATE_BILLING_CHECKOUT_SESSION_RESPONSE: CreateBillingCheckoutSessionResponse = {
  order_id: "",
  session_id: "",
  url: "",
};

export const BillingCheckoutSessionStatusSchema = z.object({
  order_id: z.string(),
  status: z.string(),
  amount_cents: z.number(),
  credits: z.number(),
  bonus_credits: z.number().default(0),
  currency: z.string().default("usd"),
  tier_id: z.string().default(""),
}).loose();

export const EMPTY_BILLING_CHECKOUT_SESSION_STATUS: BillingCheckoutSessionStatus = {
  order_id: "",
  status: "pending",
  amount_cents: 0,
  credits: 0,
  bonus_credits: 0,
  currency: "usd",
  tier_id: "",
};

export const CreateBillingPortalSessionResponseSchema = z.object({
  url: z.string(),
}).loose();

export const EMPTY_CREATE_BILLING_PORTAL_SESSION_RESPONSE: CreateBillingPortalSessionResponse = {
  url: "",
};

// ---------------------------------------------------------------------------
// Workspace subscriptions (`/api/cloud-subscriptions/*`)
//
// These schemas are the compatibility boundary with multica-cloud. Three rules
// hold for all of them:
//
//  1. There is no fallback value. Callers get `null` on any parse failure and
//     must render "unavailable" — never a synthetic Free plan, because that
//     turns an upstream outage or an older cloud into a silent downgrade of a
//     paying workspace.
//  2. `.loose()` keeps unknown keys, so a cloud that adds fields does not break
//     an older client.
//  3. `plan` and `status` stay open strings. A new plan or Stripe status must
//     surface as unknown rather than be coerced into a known one.

const WorkspaceSubscriptionIntervalSchema = z.enum(["month", "year"]);

// Stripe hosts Checkout and Portal, so those URLs leave the app. `z.string()
// .url()` is not enough on its own — `new URL("javascript:...")` parses — and
// the caller hands this value to location.assign, so the scheme is pinned here.
const StripeHostedURLSchema = z.string().url().refine(
  (value) => value.startsWith("https://"),
  { message: "Stripe hosted URL must use HTTPS" },
);

const WorkspaceEntitlementLimitSchema = z
  .discriminatedUnion("mode", [
    z
      .object({
        mode: z.literal("limited"),
        limit: z.number().int().positive(),
      })
      .loose(),
    z.object({ mode: z.literal("unlimited") }).loose(),
  ])
  .transform(
    (value): WorkspaceSubscriptionEntitlements["limits"]["issueCount"] =>
      value.mode === "limited"
        ? { mode: "limited", limit: value.limit }
        : { mode: "unlimited", limit: null },
  );

export const WorkspaceSubscriptionEntitlementsSchema = z
  .object({
    workspace_id: z.string(),
    plan: z.string(),
    status: z.string(),
    // Cloud documents seats as >= 1, but accepting 0 costs nothing and keeps a
    // workspace that momentarily reports no human members readable instead of
    // failing the whole snapshot.
    seats: z.number().int().nonnegative(),
    limits: z
      .object({
        issue_count: WorkspaceEntitlementLimitSchema,
        autopilot_runs: WorkspaceEntitlementLimitSchema,
      })
      .loose(),
    current_period_end: z.string().nullable().optional(),
    snapshot_expires_at: z.string().nullable().optional(),
    version: z.number().int().nonnegative(),
  })
  .loose()
  .transform(
    (value): WorkspaceSubscriptionEntitlements => ({
      workspaceId: value.workspace_id,
      plan: value.plan,
      status: value.status,
      seats: value.seats,
      limits: {
        issueCount: value.limits.issue_count,
        autopilotRuns: value.limits.autopilot_runs,
      },
      currentPeriodEnd: value.current_period_end ?? null,
      snapshotExpiresAt: value.snapshot_expires_at ?? null,
      version: value.version,
    }),
  );

export const WorkspaceSubscriptionSummarySchema = z
  .object({
    entitlement: WorkspaceSubscriptionEntitlementsSchema,
    billing_interval: WorkspaceSubscriptionIntervalSchema.nullable(),
    human_members: z.number().int().nonnegative(),
    seat_capacity: z
      .object({
        purchased: z.number().int().positive(),
        used: z.number().int().nonnegative(),
        reserved: z.number().int().nonnegative(),
        available: z.number().int().nonnegative(),
        overcommitted: z.boolean(),
        version: z.number().int().positive(),
        pending_quantity: z.number().int().positive().nullable(),
        active_purchase: z
          .object({
            request_id: z.string(),
            target_seats: z.number().int().positive(),
            status: z.enum(["pending", "processing", "submitted"]),
            expires_at: z.string().min(1).optional(),
          })
          .loose()
          .optional(),
      })
      .loose()
      .nullable(),
    cancel_at_period_end: z.boolean(),
    grace_until: z.string().nullable(),
    has_stripe_customer: z.boolean(),
    available_actions: z.object({
      checkout: z.boolean(),
      portal: z.boolean(),
      purchase_seats: z.boolean(),
    }).loose(),
  })
  .loose()
  .transform(
    (value): WorkspaceSubscriptionSummary => ({
      entitlement: value.entitlement,
      billingInterval: value.billing_interval,
      humanMembers: value.human_members,
      seatCapacity: value.seat_capacity
        ? {
            purchased: value.seat_capacity.purchased,
            used: value.seat_capacity.used,
            reserved: value.seat_capacity.reserved,
            available: value.seat_capacity.available,
            overcommitted: value.seat_capacity.overcommitted,
            version: value.seat_capacity.version,
            pendingQuantity: value.seat_capacity.pending_quantity,
            activePurchase: value.seat_capacity.active_purchase
              ? {
                  requestId:
                    value.seat_capacity.active_purchase.request_id,
                  targetSeats:
                    value.seat_capacity.active_purchase.target_seats,
                  status: value.seat_capacity.active_purchase.status,
                  expiresAt:
                    value.seat_capacity.active_purchase.expires_at ?? null,
                }
              : null,
          }
        : null,
      cancelAtPeriodEnd: value.cancel_at_period_end,
      graceUntil: value.grace_until,
      hasStripeCustomer: value.has_stripe_customer,
      availableActions: {
        checkout: value.available_actions.checkout,
        portal: value.available_actions.portal,
        purchaseSeats: value.available_actions.purchase_seats,
      },
    }),
  );

export const IssueLimitUsageSchema = z
  .object({
    used: z.number().int().nonnegative(),
    limit: z.number().int().positive(),
  })
  .loose()
  .transform(
    (value): IssueLimitUsage => ({
      used: value.used,
      limit: value.limit,
    }),
  );

const WorkspaceSubscriptionPriceSchema = (
  expected: "month" | "year",
) =>
  z
    .object({
      currency: z.string().min(1),
      // Reject 0 and negatives: a free or malformed Price must read as
      // "price unavailable", not as a real amount shown next to a purchase
      // button.
      unit_amount: z.number().int().positive(),
      // Pinned to the slot it arrived in. Cloud validates this too, but a
      // schema that accepted a yearly Price under `month` would let the UI
      // quote a yearly amount as a monthly one — the schema is an independent
      // boundary, so it checks the correspondence itself.
      interval: z.literal(expected),
      interval_count: z.literal(1),
    })
    .loose()
    .transform(
      (value): WorkspaceSubscriptionPrice => ({
        currency: value.currency,
        unitAmount: value.unit_amount,
        interval: value.interval,
        intervalCount: value.interval_count,
      }),
    );

export const WorkspaceSubscriptionPricesSchema = z
  .object({
    month: WorkspaceSubscriptionPriceSchema("month"),
    year: WorkspaceSubscriptionPriceSchema("year"),
  })
  .loose()
  .transform(
    (value): WorkspaceSubscriptionPrices => ({
      month: value.month,
      year: value.year,
    }),
  );

export const CreateWorkspaceSubscriptionCheckoutResponseSchema = z
  .object({
    request_id: z.string(),
    session_id: z.string(),
    url: StripeHostedURLSchema,
  })
  .loose()
  .transform(
    (value): CreateWorkspaceSubscriptionCheckoutResponse => ({
      requestId: value.request_id,
      sessionId: value.session_id,
      url: value.url,
    }),
  );

export const WorkspaceSubscriptionSeatReconcileResultSchema = z
  .object({
    workspace_id: z.string(),
    action: z.string(),
    version: z.number().int().nonnegative(),
  })
  .loose()
  .transform(
    (value): WorkspaceSubscriptionSeatReconcileResult => ({
      workspaceId: value.workspace_id,
      action: value.action,
      version: value.version,
    }),
  );

export const WorkspaceSeatPurchasePreviewSchema = z
  .object({
    current_seats: z.number().int().positive(),
    additional_seats: z.number().int().positive(),
    resulting_seats: z.number().int().positive(),
    purchase_version: z.number().int().positive(),
    currency: z.string().regex(/^[a-z]{3}$/),
    proration_amount: z.number().int().nonnegative(),
    next_invoice_amount: z.number().int().nonnegative(),
    quoted_at: z.string().min(1),
  })
  .loose()
  .transform(
    (value): WorkspaceSeatPurchasePreview => ({
      currentSeats: value.current_seats,
      additionalSeats: value.additional_seats,
      resultingSeats: value.resulting_seats,
      purchaseVersion: value.purchase_version,
      currency: value.currency,
      prorationAmount: value.proration_amount,
      nextInvoiceAmount: value.next_invoice_amount,
      quotedAt: value.quoted_at,
    }),
  );

export const PurchaseWorkspaceSeatsResponseSchema = z
  .object({
    request_id: z.string(),
    current_seats: z.number().int().positive(),
    additional_seats: z.number().int().positive(),
    resulting_seats: z.number().int().positive(),
    currency: z.string().regex(/^[a-z]{3}$/),
    proration_amount: z.number().int().nonnegative(),
    next_invoice_amount: z.number().int().nonnegative(),
    status: z.enum(["pending", "submitted", "confirmed"]),
  })
  .loose()
  .transform(
    (value): PurchaseWorkspaceSeatsResponse => ({
      requestId: value.request_id,
      currentSeats: value.current_seats,
      additionalSeats: value.additional_seats,
      resultingSeats: value.resulting_seats,
      currency: value.currency,
      prorationAmount: value.proration_amount,
      nextInvoiceAmount: value.next_invoice_amount,
      status: value.status,
    }),
  );

export const CreateWorkspaceSubscriptionPortalResponseSchema = z
  .object({
    url: StripeHostedURLSchema,
  })
  .loose()
  .transform(
    (value): CreateWorkspaceSubscriptionPortalResponse => ({
      url: value.url,
    }),
  );

// ---------------------------------------------------------------------------
// Runtime model discovery (`POST /api/runtimes/:id/models`,
// `GET /api/runtimes/:id/models/:requestId`). Both endpoints return the same
// request record, and the UI drives a state machine off `status`, so the two
// fields that decide behaviour are pinned: `status` gates the polling loop and
// `supported` gates whether the picker is usable at all. Everything else stays
// lenient per the rules at the top of this file.
//
// `status` deliberately stays `z.string()` (a newer server may add a state);
// `resolveRuntimeModels` treats anything it does not recognise as an explicit
// failure rather than a completed-but-empty catalog. `supported` defaults to
// true so a server old enough to omit it keeps the picker enabled instead of
// rendering "managed by runtime" off an `undefined`.
//
// `cached` / `cached_at` are additive markers for a snapshot served from the
// server-side catalog cache (MUL-5444); an older backend omits them.
// ---------------------------------------------------------------------------

const RuntimeModelThinkingLevelSchema = z.object({
  value: z.string(),
  label: z.string().default(""),
  description: z.string().optional(),
}).loose();

const RuntimeModelThinkingSchema = z.object({
  supported_levels: z.array(RuntimeModelThinkingLevelSchema).default([]),
  default_level: z.string().optional(),
}).loose();

const RuntimeModelServiceTierSchema = z.object({
  id: z.string(),
  name: z.string().default(""),
  description: z.string().optional(),
}).loose();

// A model entry with no `id` is unselectable — `onChange(m.id)` would persist
// an empty model — so `id` is required and a malformed entry drops the whole
// response to the fallback rather than rendering a dead row.
const RuntimeModelSchema = z.object({
  id: z.string(),
  label: z.string().default(""),
  provider: z.string().optional(),
  default: z.boolean().optional(),
  thinking: RuntimeModelThinkingSchema.nullable().optional()
    .transform((v) => v ?? undefined),
  service_tiers: z.array(RuntimeModelServiceTierSchema).optional(),
  supports_explicit_standard_service_tier: z.boolean().optional(),
}).loose();

// A row the runtime named but will not run (MUL-6961). Parsed from its own
// top-level list, never from `models`, so nothing here can become a selectable
// value. `id` is required for the same reason it is on RuntimeModelSchema — a
// row without one cannot even be keyed in a list.
const RuntimeUnavailableModelSchema = z.object({
  id: z.string(),
  label: z.string().default(""),
  reason: z.string().optional(),
}).loose();

export const RuntimeModelListRequestSchema = z.object({
  id: z.string().default(""),
  runtime_id: z.string().default(""),
  status: z.string(),
  models: z.array(RuntimeModelSchema).optional(),
  // Absent on any daemon or server older than the field, which simply means
  // the picker shows no unavailable section.
  unavailable_models: z.array(RuntimeUnavailableModelSchema).optional(),
  supported: z.boolean().default(true),
  error: z.string().optional(),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
  cached: z.boolean().optional(),
  cached_at: z.string().optional(),
}).loose();

// Fallback for an unparseable model-discovery response. `failed` is the only
// honest choice: `completed` would fabricate an empty catalog (and silently
// clear a saved model when `supported` is read as false), while `pending`
// would spin the picker until the client-side poll timeout. `failed` surfaces
// "discovery failed" immediately and leaves the creatable manual-entry field
// working, which is the same degradation as a real discovery failure.
export const MALFORMED_RUNTIME_MODEL_LIST_REQUEST: RuntimeModelListRequest = {
  id: "",
  runtime_id: "",
  status: "failed",
  supported: true,
  error: "invalid model discovery response",
  created_at: "",
  updated_at: "",
};

const RuntimeDirectoryEntrySchema = z.object({
  name: z.string().catch(""),
  path: z.string().catch(""),
  kind: z.enum(["file", "folder"]).catch("file"),
  hidden: z.boolean().catch(false),
}).loose();

export const RuntimeDirectoryListingRequestSchema = z.object({
  id: z.string().catch(""),
  runtime_id: z.string().catch(""),
  status: z.enum(["pending", "running", "completed", "failed", "timeout"])
    .catch("failed"),
  path: z.string().catch(""),
  entries: z.array(RuntimeDirectoryEntrySchema).catch([]),
  error: z.string().catch(""),
  created_at: z.string().catch(""),
  updated_at: z.string().catch(""),
}).loose();

export const MALFORMED_RUNTIME_DIRECTORY_LISTING_REQUEST: RuntimeDirectoryListingRequest = {
  id: "",
  runtime_id: "",
  status: "failed",
  path: "",
  entries: [],
  error: "invalid directory listing response",
  created_at: "",
  updated_at: "",
};

export const DingTalkInstallationSchema = z.object({
  id: z.string(),
  workspace_id: z.string().default(""),
  agent_id: z.string().default(""),
  installer_user_id: z.string().default(""),
  status: z.string().default("revoked"),
  installed_at: z.string().default(""),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
  agent_available: z.boolean().optional(),
  bound_dingtalk_user_ids: z.array(z.string()).catch([]).default([]),
}).loose();

export const EMPTY_DINGTALK_INSTALLATION: DingTalkInstallation = {
  id: "",
  workspace_id: "",
  agent_id: "",
  installer_user_id: "",
  status: "revoked",
  installed_at: "",
  created_at: "",
  updated_at: "",
  bound_dingtalk_user_ids: [],
};

export const ListDingTalkInstallationsResponseSchema = z.object({
  installations: z.array(DingTalkInstallationSchema).default([]),
  configured: z.boolean().default(false),
  install_supported: z.boolean().optional(),
}).loose();

export const EMPTY_LIST_DINGTALK_INSTALLATIONS_RESPONSE: ListDingTalkInstallationsResponse = {
  installations: [],
  configured: false,
};

export const DingTalkGroupBotSchema = z.object({
  installation_id: z.string().default(""),
  agent_id: z.string().default(""),
  bot_name: z.string().default(""),
  bot_identity_issue: z.string().default(""),
  last_active_at: z.string().optional(),
  mention_count: z.number().int().nonnegative().optional(),
}).loose();

export const DingTalkGroupSchema = z.object({
  conversation_id: z.string(),
  conversation_title: z.string().default(""),
  bots: z.array(DingTalkGroupBotSchema).catch([]).default([]),
}).loose();

export const ListDingTalkGroupsResponseSchema = z.object({
  groups: z.array(DingTalkGroupSchema).default([]),
  group_discovery_supported: z.boolean().default(false),
  inactive_group_counts: z.record(z.string(), z.number().int().nonnegative()).optional(),
  bot_identities: z.record(z.string(), DingTalkGroupBotSchema).optional(),
  next_offset: z.number().int().nonnegative().optional(),
}).loose();

export const EMPTY_LIST_DINGTALK_GROUPS_RESPONSE: ListDingTalkGroupsResponse = {
  groups: [],
  group_discovery_supported: false,
};

export const RedeemDingTalkBindingTokenResponseSchema = z.object({
  workspace_id: z.string().default(""),
  installation_id: z.string().default(""),
  dingtalk_user_id: z.string().default(""),
}).loose();

export const EMPTY_REDEEM_DINGTALK_BINDING_TOKEN_RESPONSE: RedeemDingTalkBindingTokenResponse = {
  workspace_id: "",
  installation_id: "",
  dingtalk_user_id: "",
};

// WeCom smart-bot ("智能机器人" / aibot) installation responses. `.loose()` so a
// newer backend field never fails the parse on an older desktop build (see
// CLAUDE.md → API Compatibility). Defaults are chosen so a malformed response
// degrades safely: `configured` defaults false (renders the "ask your operator"
// state rather than a Connect dialog whose submit is guaranteed to fail), and a
// missing `status` defaults to "revoked" rather than "active" so a broken read
// never shows a bot as connected when it may not be.
export const WecomInstallationSchema = z.object({
  id: z.string(),
  workspace_id: z.string().default(""),
  agent_id: z.string().default(""),
  bot_id: z.string().default(""),
  installer_user_id: z.string().default(""),
  status: z.string().default("revoked"),
}).loose();

export const EMPTY_WECOM_INSTALLATION: WecomInstallation = {
  id: "",
  workspace_id: "",
  agent_id: "",
  bot_id: "",
  installer_user_id: "",
  status: "revoked",
};

export const ListWecomInstallationsResponseSchema = z.object({
  installations: z.array(WecomInstallationSchema).default([]),
  configured: z.boolean().default(false),
  install_supported: z.boolean().optional(),
}).loose();

export const EMPTY_LIST_WECOM_INSTALLATIONS_RESPONSE: ListWecomInstallationsResponse = {
  installations: [],
  configured: false,
};

export const RedeemWecomBindingTokenResponseSchema = z.object({
  workspace_id: z.string().default(""),
  installation_id: z.string().default(""),
  wecom_user_id: z.string().default(""),
}).loose();

export const EMPTY_REDEEM_WECOM_BINDING_TOKEN_RESPONSE: RedeemWecomBindingTokenResponse = {
  workspace_id: "",
  installation_id: "",
  wecom_user_id: "",
};

export const TelegramInstallationSchema = z.object({
  id: z.string(),
  workspace_id: z.string().default(""),
  agent_id: z.string().default(""),
  bot_id: z.string().default(""),
  bot_username: z.string().default(""),
  installer_user_id: z.string().default(""),
  status: z.string().default("revoked"),
  installed_at: z.string().default(""),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const EMPTY_TELEGRAM_INSTALLATION: TelegramInstallation = {
  id: "",
  workspace_id: "",
  agent_id: "",
  bot_id: "",
  bot_username: "",
  installer_user_id: "",
  status: "revoked",
  installed_at: "",
  created_at: "",
  updated_at: "",
};

export const ListTelegramInstallationsResponseSchema = z.object({
  installations: z.array(TelegramInstallationSchema).default([]),
  configured: z.boolean().default(false),
  install_supported: z.boolean().optional(),
}).loose();

export const EMPTY_LIST_TELEGRAM_INSTALLATIONS_RESPONSE: ListTelegramInstallationsResponse = {
  installations: [],
  configured: false,
};

export const RedeemTelegramBindingTokenResponseSchema = z.object({
  workspace_id: z.string().default(""),
  installation_id: z.string().default(""),
  telegram_user_id: z.string().default(""),
}).loose();

export const EMPTY_REDEEM_TELEGRAM_BINDING_TOKEN_RESPONSE: RedeemTelegramBindingTokenResponse = {
  workspace_id: "",
  installation_id: "",
  telegram_user_id: "",
};

// Skills. Introduced for `POST /api/skills/:id/refresh` (update a skill from
// its imported source). `config` stays a loose record: the server owns the
// `origin` provenance shape and may extend it freely.
export const SkillFileSchema = z.object({
  id: z.string(),
  skill_id: z.string(),
  path: z.string(),
  content: z.string().optional().default(""),
  created_at: z.string().optional().default(""),
  updated_at: z.string().optional().default(""),
}).loose();

// Workspace list shape (`GET /api/skills`). Intentionally no `content` /
// `files`: those belong on the detail endpoint. Defaulting them here used
// to invent empty bodies on every list row (GH #2174).
export const SkillSummarySchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  name: z.string(),
  description: z.string().optional().default(""),
  config: z.record(z.string(), z.unknown()).optional().default({}),
  created_by: z.string().nullable().optional().default(null),
  created_at: z.string().optional().default(""),
  updated_at: z.string().optional().default(""),
  enabled: z.boolean().optional(),
  // Catch-to-empty so a missing/malformed labels field cannot fail the
  // whole skill (or the list it lives in). Older backends omit it; the
  // filter treats an empty array as "no labels".
  labels: z.array(LabelSchema).catch([]),
}).loose();

export const EMPTY_SKILL_SUMMARY: SkillSummary = {
  id: "",
  workspace_id: "",
  name: "",
  description: "",
  config: {},
  created_by: null,
  created_at: "",
  updated_at: "",
  labels: [],
};

export const SkillSchema = SkillSummarySchema.extend({
  content: z.string().optional().default(""),
  files: z.array(SkillFileSchema).optional().default([]),
}).loose();

export const EMPTY_SKILL: Skill = {
  ...EMPTY_SKILL_SUMMARY,
  content: "",
  files: [],
};

export const SkillSummaryListSchema = z.array(SkillSummarySchema).default([]);

export const EMPTY_SKILL_SUMMARY_LIST: SkillSummary[] = [];

export const SkillImportExistingSkillSchema = z.object({
  id: z.string(),
  name: z.string(),
  created_by: z.string().optional(),
  can_overwrite: z.boolean().optional(),
}).loose();

/**
 * Envelope of POST /api/skills/import.
 *
 * `status` stays a plain string (not an enum) so a status added by a newer
 * backend still parses and its `reason` survives to the user. `z.enum` here
 * would fail the whole envelope on an unknown value, drop the server's reason
 * and leave only a generic "Import failed" — the server field is a bare
 * `string`, so it is free to grow. `skillFromImportResult` has the default
 * branch: anything outside created/updated is treated as a failure.
 */
export const SkillImportResultSchema = z.object({
  status: z.string().default("failed"),
  reason: z.string().optional().default(""),
  skill: SkillSchema.optional(),
  existing_skill: SkillImportExistingSkillSchema.optional(),
}).loose();

export const EMPTY_SKILL_IMPORT_RESULT: SkillImportResult = {
  status: "failed",
  reason: "",
};

/**
 * Read shape of one workspace MCP server.
 *
 * This is the ONLY schema in this file that must not be `.loose()`. Everywhere
 * else, keeping unknown fields is forward-compatibility; here it would be a
 * hole in the write-only boundary — a server that regressed to returning the
 * stored entry (or a `url` / `headers` on the summary) would have it land in
 * the parsed object and in the query cache. zod strips unknown keys by
 * default, so the client only ever holds the safe summary.
 *
 * `transport` stays a plain string (not an enum) so an unknown value from a
 * newer backend still parses — the UI has a default branch for it.
 */
export const WorkspaceMcpServerSchema = z.object({
  id: z.string().default(""),
  workspace_id: z.string().default(""),
  name: z.string().default(""),
  transport: z.string().default("unknown"),
  enabled: z.boolean().optional(),
  // Older servers omit it; a malformed value drops to "unknown" rather than
  // failing the whole list.
  agent_count: z.number().int().nonnegative().optional().catch(undefined),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
});

export const WorkspaceMcpServerListSchema = z.array(WorkspaceMcpServerSchema);

export const EMPTY_WORKSPACE_MCP_SERVER: WorkspaceMcpServer = {
  id: "",
  workspace_id: "",
  name: "",
  transport: "unknown",
  created_at: "",
  updated_at: "",
};

// Share links. Introduced with the workspace share-link invite flow; schemas
// mirror the API responses so malformed payloads fall back to safe defaults.
export const ShareLinkSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  code: z.string(),
  created_by: z.string(),
  role: z.string(),
  expires_at: z.string().nullable().optional().default(null),
  max_uses: z.number().nullable().optional().default(null),
  use_count: z.number().optional().default(0),
  is_active: z.boolean().optional().default(true),
  created_at: z.string().optional().default(""),
  creator_name: z.string().optional().default(""),
  creator_email: z.string().optional().default(""),
}).loose();

export const EMPTY_SHARE_LINK: ShareLink = {
  id: "",
  workspace_id: "",
  code: "",
  created_by: "",
  role: "member",
  expires_at: null,
  max_uses: null,
  use_count: 0,
  is_active: false,
  created_at: "",
};

export const ShareLinkListResponseSchema = z.array(ShareLinkSchema).default([]);

export const ShareLinkInfoSchema = z.object({
  workspace_name: z.string().optional().default(""),
  workspace_slug: z.string().optional().default(""),
  creator_name: z.string().optional().default(""),
  role: z.string().optional().default("member"),
}).loose();

export const EMPTY_SHARE_LINK_INFO: ShareLinkInfo = {
  workspace_name: "",
  workspace_slug: "",
  role: "member",
};

export const MemberWithUserSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  user_id: z.string(),
  role: z.string(),
  created_at: z.string().optional().default(""),
  name: z.string().optional().default(""),
  email: z.string().optional().default(""),
  avatar_url: z.string().nullable().optional().default(null),
}).loose();

export const JoinShareLinkResponseSchema = z.object({
  member: MemberWithUserSchema,
  workspace_id: z.string(),
  workspace_slug: z.string().optional().default(""),
}).loose();

export const EMPTY_JOIN_SHARE_LINK_RESPONSE: {
  member: MemberWithUser;
  workspace_id: string;
  workspace_slug: string;
} = {
  member: {
    id: "",
    workspace_id: "",
    user_id: "",
    role: "member",
    created_at: "",
    name: "",
    email: "",
    avatar_url: null,
  },
  workspace_id: "",
  workspace_slug: "",
};

export const WakeupConditionSchema = z.union([
  z.object({ type: z.literal("issue_field"), field: z.literal("status"), value: z.string() }),
  z.object({ type: z.literal("issue_field"), field: z.literal("assignee"), assignee_type: z.enum(["member", "agent", "squad"]), assignee_id: z.string() }),
  z.object({ type: z.literal("issue_field"), field: z.literal("label"), label_id: z.string() }),
  z.object({ type: z.literal("issue_field"), field: z.literal("property"), property_id: z.string(), value: z.unknown() }),
  z.object({ type: z.literal("children_done"), stage: z.number().int().nullish() }),
  z.object({ type: z.literal("pull_request"), event: z.enum(["checks_finished", "merged"]) }),
  z.object({ type: z.literal("other_issue"), issue_id: z.string(), state: z.enum(["done", "ended", "in_review"]), identifier: z.string().optional() }),
]);

export const WakeupRunSchema = z.object({
  id: z.string(), status: z.string(), created_at: z.string(),
  started_at: z.string().nullable(), completed_at: z.string().nullable(),
  checkin_note: z.string().default(""), triggers: z.array(z.string()).default([]),
  commented: z.boolean().default(false),
});

export const PausedWakeupSchema = z.object({
  issue_id: z.string(), id: z.string(), agent_id: z.string(),
  paused_reason: z.enum(["max_fires", "loop", "rate"]).catch("rate"),
});

export const IssueWakeupSchema = z.object({
  id: z.string(), issue_id: z.string(), agent_id: z.string(), agent_name: z.string().default(""),
  instruction: z.string(), kind: z.enum(["event", "at", "every", "cron"]), mode: z.enum(["once", "continuous"]),
  event_types: z.array(z.string()), filter_agent_id: z.string().nullable(), filter_task_id: z.string().nullable(),
  interval_seconds: z.number().nullable(), cron_expression: z.string().nullable(), timezone: z.string(),
  next_fire_at: z.string().nullable(), enabled: z.boolean(), disabled_at: z.string().nullable(),
  last_task_id: z.string().nullable(), last_error: z.string().nullable(),
  filter_actor_type: z.enum(["member", "agent"]).nullable().optional(),
  filter_actor_id: z.string().nullable().optional(),
  filter_actor_name: z.string().nullable().optional(),
  revision: z.number().int().positive().optional(),
  filter_agent_name: z.string().nullable().optional(), last_task_status: z.string().nullable().optional(),
  expires_at: z.string().nullish(), expiry_seconds: z.number().nullish(),
  on_timeout: z.enum(["wake", "end"]).nullish().catch(null), timed_out_at: z.string().nullish(),
  created_by_agent: z.boolean().optional(), created_by_name: z.string().nullish(),
  source_agent_id: z.string().nullish(), source_agent_name: z.string().nullish(),
  // An unknown condition shape from a newer server reads as "no condition".
  condition: WakeupConditionSchema.nullish().catch(null),
  max_fires: z.number().int().nullish(), fire_count: z.number().int().nonnegative().optional().catch(undefined),
  paused_reason: z.enum(["max_fires", "loop", "rate"]).nullish().catch(null),
});

export const SystemWakeupSchema = z.object({
  id: z.string().default(""), revision: z.number().int().nonnegative().default(0),
  rule: z.literal("child_done"), enabled: z.boolean(), instruction: z.string().default(""),
  default_instruction: z.string().default(""), customized: z.boolean().default(false),
  paused_reason: z.enum(["max_fires", "loop", "rate"]).nullish().catch(null).transform((v) => v ?? null),
  staged: z.boolean(), stage: z.number().int().nullable(), total: z.number().int().nonnegative(),
  remaining: z.number().int().nonnegative(), waiting: z.array(z.string()).default([]),
  target: z.object({ type: z.enum(["agent", "squad", "member"]), id: z.string(), name: z.string() }).nullable().catch(null),
  blocked: z.enum(["", "backlog", "member_assignee", "no_assignee"]).catch(""),
  workspace_default: z.boolean().default(true),
});

export const WorkspaceSystemWakeupSchema = z.object({
  rule: z.literal("child_done"), enabled: z.boolean().default(true), instruction: z.string().default(""),
  builtin_instruction: z.string().default(""), customized: z.number().int().nonnegative().default(0),
});

export const IssueWakeupSummaryRowSchema = IssueWakeupSchema.pick({
  id: true, issue_id: true, agent_id: true, agent_name: true, kind: true, mode: true,
  event_types: true, filter_task_id: true, filter_agent_name: true, interval_seconds: true,
  filter_actor_type: true, filter_actor_id: true, filter_actor_name: true,
  cron_expression: true, timezone: true, next_fire_at: true, condition: true,
}).extend({ active_count: z.number().int().positive(), event_count: z.number().int().nonnegative() });

export const WorkspaceWakeupPageSchema = z.object({
  items: z.array(IssueWakeupSchema.omit({ instruction: true }).extend({
    issue_title: z.string(), issue_identifier: z.string(), issue_closed: z.boolean(),
    can_manage: z.boolean(), active_runs: z.number().int().nonnegative(),
    task: AgentTaskSchema.nullable(),
    // System rule rows have no target when the issue has no agent assignee,
    // and no revision.
    agent_id: z.string().nullish().transform((v) => v ?? ""),
    revision: z.number().int().positive().nullish().catch(undefined).transform((v) => v ?? undefined),
    source: z.enum(["member", "agent", "system"]).catch("member").default("member"),
    runs_7d: z.number().int().nonnegative().default(0),
    rule: z.literal("child_done").nullish().catch(null),
    system_stage: z.number().int().nullish(), system_remaining: z.number().int().nullish(),
    target_type: z.string().nullish(),
  })),
  total: z.number().int().nonnegative(),
  counts: z.object({
    active: z.number().int().nonnegative(), all: z.number().int().nonnegative(),
    paused: z.number().int().nonnegative().default(0),
    disabled: z.number().int().nonnegative(), ended: z.number().int().nonnegative(),
  }),
  agents: z.array(z.object({ id: z.string(), name: z.string() })),
});

// Older servers omit runtime_type; the protocol remains their compatibility target.
export const RuntimeProfileSchema = z
  .object({
    id: z.string(),
    workspace_id: z.string(),
    display_name: z.string(),
    protocol_family: z.string(),
    runtime_type: z.string().nullish().catch(undefined),
    command_name: z.string(),
    description: z.string().nullable().catch(null),
    fixed_args: z.array(z.string()).catch([]),
    visibility: z.string().catch("workspace"),
    created_by: z.string().nullable().catch(null),
    enabled: z.boolean().catch(true),
    created_at: z.string().catch(""),
    updated_at: z.string().catch(""),
  })
  .passthrough()
  .transform((profile) => ({
    ...profile,
    runtime_type: profile.runtime_type || profile.protocol_family,
  }));
export const RuntimeProfileListSchema = z.array(RuntimeProfileSchema);

// ---------------------------------------------------------------------------
// Questions (`/api/questions/*`, `GET /api/issues/{id}/questions`).
//
// The core of a question must parse: ids, prompt, answerers and answers. The
// rest degrades on its own, so an installed client keeps showing questions a
// newer server sends:
//   - an unknown `status` reads as `closed`, which offers no controls;
//   - a question without its ask fields, as a server before question sets
//     sends it, is its own one-question ask: `ask_id` reads as its `id`,
//     `ask_position` as 0, `ask_title` as null, and a missing or unknown
//     `answer_policy` as `any_one`;
//   - missing wake targets read as an empty list; malformed targets are
//     dropped individually;
//   - a link or event this client does not know, or a malformed one, is
//     dropped from its list;
//   - an answer type, settings, suggested answer or answer value this client
//     cannot read turns the question into `answer_type: "unknown"`, which
//     keeps the prompt and each answer's `display_text` and offers no controls.
// A list drops a malformed question instead of hiding the others.
// ---------------------------------------------------------------------------

// Absent and null both read as "not given" for an optional field.
const optionalQuestionField = <T extends z.ZodType>(schema: T) =>
  schema.nullish().transform((value) => value ?? undefined);

const QuestionIdListSchema = z.array(z.string());

interface AnswerTypeSchemas {
  settings: z.ZodType;
  value: z.ZodType;
}

// The settings and value shapes of each answer type (design.md decision 2).
// Shape only: the rules between them (ranges, min/max, notes) are
// `questions/validate.ts`'s and the server's. A missing boolean setting reads
// false.
const ANSWER_TYPE_SCHEMAS = {
  text: {
    settings: z.object({
      length: z.enum(["short", "long"]),
      placeholder: optionalQuestionField(z.string()),
    }).loose(),
    value: z.object({
      text: z.string(),
      attachment_ids: optionalQuestionField(QuestionIdListSchema),
    }).loose(),
  },
  yes_no: {
    settings: z.object({ why_on_no: z.boolean().default(false) }).loose(),
    value: z.object({ choice: z.enum(["yes", "no"]) }).loose(),
  },
  single_choice: {
    settings: z.object({
      options: z.array(z.string()),
      allow_other: z.boolean().default(false),
    }).loose(),
    value: z.union([
      z.object({ option: z.string() }).loose(),
      z.object({ other: z.string() }).loose(),
    ]),
  },
  multiple_choice: {
    settings: z.object({
      options: z.array(z.string()),
      min: z.number().int(),
      max: z.number().int(),
    }).loose(),
    value: z.object({ options: z.array(z.string()) }).loose(),
  },
  confirm_facts: {
    settings: z.object({ facts: z.array(z.string()) }).loose(),
    value: z.object({
      facts: z.array(z.union([
        z.object({ correct: z.literal(true) }).loose(),
        z.object({ correct: z.literal(false), correction: z.string() }).loose(),
      ])),
    }).loose(),
  },
  ranking: {
    settings: z.object({ items: z.array(z.string()) }).loose(),
    value: z.object({ order: z.array(z.string()) }).loose(),
  },
  scale: {
    settings: z.object({
      min: z.number().int(),
      max: z.number().int(),
      min_label: optionalQuestionField(z.string()),
      max_label: optionalQuestionField(z.string()),
    }).loose(),
    value: z.object({ point: z.number().int() }).loose(),
  },
  number: {
    settings: z.object({
      unit: optionalQuestionField(z.string()),
      min: optionalQuestionField(z.number()),
      max: optionalQuestionField(z.number()),
    }).loose(),
    value: z.object({ number: z.number() }).loose(),
  },
  date: {
    settings: z.object({ kind: z.enum(["date", "datetime", "range"]) }).loose(),
    value: z.union([
      z.object({ start: z.string(), end: z.string() }).loose(),
      z.object({ datetime: z.string() }).loose(),
      z.object({ date: z.string() }).loose(),
    ]),
  },
  person: {
    settings: z.object({ allow: z.enum(["members", "agents", "both"]) }).loose(),
    value: z.object({ type: z.enum(["member", "agent"]), id: z.string() }).loose(),
  },
  file: {
    settings: z.object({ accept: z.array(z.string()) }).loose(),
    value: z.object({ attachment_ids: QuestionIdListSchema }).loose(),
  },
  approval: {
    settings: z.object({
      subject: z.string(),
      link: optionalQuestionField(z.object({ title: z.string(), url: z.string() }).loose()),
    }).loose(),
    value: z.object({ decision: z.enum(["approve", "request_changes", "decline"]) }).loose(),
  },
} satisfies Record<AnswerType, AnswerTypeSchemas>;

const isAnswerType = (value: string): value is AnswerType =>
  Object.hasOwn(ANSWER_TYPE_SCHEMAS, value);

// Parses each entry on its own and drops the ones that do not fit; a missing
// or non-list value reads as an empty list.
const listDroppingUnreadable = <T extends z.ZodType>(item: T) =>
  z.array(z.unknown()).catch([]).transform((entries) =>
    entries.flatMap((entry) => {
      const parsed = item.safeParse(entry);
      return parsed.success ? [parsed.data as z.output<T>] : [];
    }),
  );

const QuestionLinkSchema = z.object({
  link_type: z.enum(["issue", "task", "comment", "project", "agent", "squad", "autopilot", "skill"]),
  link_id: z.string(),
  position: z.number().int(),
}).loose();

const QuestionWakeTargetSchema = z.object({
  agent_id: z.string().min(1),
  issue_id: z.string().min(1),
  unseen_answer_count: z.number().int().nonnegative().catch(0),
  wake_available: BooleanWithDefaultSchema(false),
}).loose();

const QuestionViaSchema = z.object({
  type: z.enum(["issue", "sub_issue", "comment", "task"]).catch("issue"),
  id: z.string().min(1),
  identifier: z.string().nullish().catch(null).transform((value) => value ?? null),
  issue_identifier: z.string().nullish().catch(null).transform((value) => value ?? null),
}).loose();

const QuestionEventShape = {
  id: z.string(),
  actor_type: z.enum(["member", "agent"]),
  actor_id: z.string().nullish().transform((value) => value ?? null),
  created_at: z.string(),
};

const QuestionEventSchema = z.discriminatedUnion("kind", [
  z.object({
    ...QuestionEventShape,
    kind: z.literal(["asked", "answered", "answer_edited", "closed", "withdrawn"]),
    details: z.unknown().transform((): Record<string, never> => ({})),
  }).loose(),
  z.object({
    ...QuestionEventShape,
    kind: z.literal("ask_back"),
    details: z.object({ text: z.string() }).loose(),
  }).loose(),
  z.object({
    ...QuestionEventShape,
    kind: z.literal("reassigned"),
    details: z.object({ user_id: z.string(), note: z.string() }).loose(),
  }).loose(),
  z.object({
    ...QuestionEventShape,
    kind: z.literal("woken"),
    details: z.object({
      task_id: z.string(),
      answer_count: z.number().int().nonnegative(),
    }).loose(),
  }).loose(),
]);

const QuestionAnswererSchema = z.object({
  user_id: z.string(),
  assigned_at: z.string().catch(""),
}).loose();

const QuestionAnswerWireSchema = z.object({
  id: z.string(),
  answerer_user_id: z.string(),
  value: z.unknown(),
  display_text: z.string().catch(""),
  note: z.string().nullish().catch(null).transform((value) => value ?? null),
  created_at: z.string(),
  updated_at: z.string(),
}).loose();

const QuestionWireSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  asker_agent_id: z.string(),
  asked_in_task_id: z.string(),
  ask_id: z.string().min(1).nullish().catch(null),
  ask_position: z.number().int().nonnegative().catch(0),
  ask_title: z.string().nullish().catch(null).transform((value) => value ?? null),
  answer_policy: z.enum(["any_one", "each"]).catch("any_one"),
  prompt: z.string(),
  context_line: z.string().nullish().catch(null).transform((value) => value ?? null),
  answer_type: z.string(),
  settings: z.unknown(),
  suggested_answer: z.unknown(),
  status: z.enum(["open", "partly_answered", "answered", "closed", "withdrawn"]).catch("closed"),
  due_date: z.string().nullish().catch(null).transform((value) => value ?? null),
  blocks_work: BooleanWithDefaultSchema(false),
  links: listDroppingUnreadable(QuestionLinkSchema),
  answerers: z.array(QuestionAnswererSchema).default([]),
  answers: z.array(QuestionAnswerWireSchema).default([]),
  events: listDroppingUnreadable(QuestionEventSchema),
  unseen_answer_count: z.number().int().nonnegative().catch(0),
  wake_available: BooleanWithDefaultSchema(false),
  wake_targets: listDroppingUnreadable(QuestionWakeTargetSchema),
  created_at: z.string(),
  updated_at: z.string(),
}).loose();

// Types the answer-type parts when every one of them fits its type, else
// reads the question as "unknown".
function readQuestion({
  ask_id,
  answer_type,
  settings,
  suggested_answer,
  answers,
  ...wire
}: z.output<typeof QuestionWireSchema>): Question {
  const rest = { ...wire, ask_id: ask_id ?? wire.id };
  if (isAnswerType(answer_type)) {
    const schemas: AnswerTypeSchemas = ANSWER_TYPE_SCHEMAS[answer_type];
    const parsedSettings = schemas.settings.safeParse(settings);
    const parsedSuggestion =
      suggested_answer == null ? null : schemas.value.safeParse(suggested_answer);
    const parsedValues = answers.map((answer) => schemas.value.safeParse(answer.value));
    if (
      parsedSettings.success &&
      parsedSuggestion?.success !== false &&
      parsedValues.every((parsed) => parsed.success)
    ) {
      // The schemas above fit AnswerSettingsByType / AnswerValueByType of
      // `answer_type`; TS cannot correlate the lookup with the union.
      return {
        ...rest,
        answer_type,
        settings: parsedSettings.data,
        suggested_answer: parsedSuggestion?.data ?? null,
        answers: answers.map((answer, index) => ({ ...answer, value: parsedValues[index]?.data })),
      } as Question;
    }
  }
  const unreadable: UnreadableQuestion = {
    ...rest,
    answer_type: "unknown",
    settings:
      typeof settings === "object" && settings !== null && !Array.isArray(settings)
        ? (settings as Record<string, unknown>)
        : {},
    suggested_answer: null,
    answers: answers.map((answer) => ({ ...answer, value: answer.value })),
  };
  return unreadable;
}

export const QuestionSchema = QuestionWireSchema.transform(readQuestion);

export const EMPTY_QUESTION: Question = {
  id: "",
  workspace_id: "",
  asker_agent_id: "",
  asked_in_task_id: "",
  ask_id: "",
  ask_position: 0,
  ask_title: null,
  answer_policy: "any_one",
  prompt: "",
  context_line: null,
  answer_type: "unknown",
  settings: {},
  suggested_answer: null,
  status: "closed",
  due_date: null,
  blocks_work: false,
  links: [],
  answerers: [],
  answers: [],
  events: [],
  unseen_answer_count: 0,
  wake_available: false,
  wake_targets: [],
  created_at: "",
  updated_at: "",
};

export const QuestionListSchema = z
  .array(QuestionSchema.catch(EMPTY_QUESTION))
  .transform((questions) => questions.filter((question) => question.id !== ""))
  .default([]);

export const EMPTY_QUESTION_LIST: Question[] = [];

// `GET`/`PUT /api/agent-questions`: the workspace Questions default. Missing
// fields fall back to On with no issue of its own and no brief text.
export const AgentQuestionsSettingsSchema = z.object({
  enabled: z.boolean().catch(true).default(true),
  own_setting_count: z.number().int().nonnegative().catch(0).default(0),
  brief_section: z.string().catch("").default(""),
});

export const EMPTY_AGENT_QUESTIONS_SETTINGS: AgentQuestionsSettings = {
  enabled: true,
  own_setting_count: 0,
  brief_section: "",
};

// 202 of `POST /api/questions/{id}/wake`: the queued run and the answers it carries.
export const QuestionWakeResultSchema = z.object({
  task_id: z.string().min(1),
  answer_count: z.number().int().nonnegative().catch(0),
}).loose();

export const EMPTY_QUESTION_WAKE_RESULT: QuestionWakeResult = {
  task_id: "",
  answer_count: 0,
};

// ---------------------------------------------------------------------------
// Question rows (`GET /api/questions`, `GET /api/questions/open-count`).
//
// A row reads as leniently as a question: an unknown `status` reads as
// `closed`, a row without its ask fields as its own one-question ask, a
// missing or unknown `answer_policy` as `any_one`, and an answer type or
// settings this client cannot read as `answer_type: "unknown"`. A row must
// carry its ids, prompt and first link's page object; a page drops a row
// without them instead of hiding the others. Non-issue objects may have no
// identifier or status. Unknown link and via types read as `issue`; absent
// via reads as null, absent open_issue as the legacy issue link. A page
// without `rows` reads as empty, a missing `next_cursor` as the last page,
// missing or malformed `my_answer` as not answered, and malformed counts as 0.
// ---------------------------------------------------------------------------

const QuestionRowLinkSchema = z.object({
  type: z.enum(["issue", "project", "agent", "squad", "autopilot", "skill"]).catch("issue"),
  id: z.string().min(1),
  identifier: z.string().min(1).nullish().transform((value) => value ?? null),
  title: z.string().catch(""),
  status: z.string().nullish().catch(null).transform((value) => value ?? null),
}).loose().refine((link) => link.type !== "issue" || link.identifier !== null, {
  message: "an issue link requires its identifier",
});

const QuestionOpenIssueSchema = z.object({
  id: z.string().min(1),
  identifier: z.string().min(1),
});

const QuestionRowAnswerSchema = z.object({
  display: z.string().catch(""),
  updated_at: z.string().catch(""),
}).loose();

const QuestionRowWireSchema = z.object({
  id: z.string(),
  ask_id: z.string().min(1).nullish().catch(null),
  ask_position: z.number().int().nonnegative().catch(0),
  ask_title: z.string().nullish().catch(null).transform((value) => value ?? null),
  prompt: z.string(),
  answer_type: z.string(),
  settings: z.unknown(),
  status: z.enum(["open", "partly_answered", "answered", "closed", "withdrawn"]).catch("closed"),
  answer_policy: z.enum(["any_one", "each"]).catch("any_one"),
  asker_agent_id: z.string(),
  answerer_ids: z.array(z.string()).default([]),
  due_date: z.string().nullish().catch(null).transform((value) => value ?? null),
  blocks_work: BooleanWithDefaultSchema(false),
  created_at: z.string(),
  link: QuestionRowLinkSchema,
  via: QuestionViaSchema.nullish().catch(null).transform((value) => value ?? null),
  open_issue: QuestionOpenIssueSchema.nullish(),
  my_answer: QuestionRowAnswerSchema.nullish().catch(null).transform((value) => value ?? null),
}).loose();

// Types `settings` when they fit the row's answer type, else reads the row
// as "unknown", as `readQuestion` does.
function readQuestionRow({
  ask_id,
  open_issue,
  answer_type,
  settings,
  ...wire
}: z.output<typeof QuestionRowWireSchema>, ctx: z.RefinementCtx): QuestionRow {
  if (open_issue == null) {
    const legacyIssue = QuestionOpenIssueSchema.safeParse(wire.link);
    if (!legacyIssue.success) {
      ctx.addIssue({ code: "custom", message: "question row has no opening issue" });
      return z.NEVER;
    }
    open_issue = legacyIssue.data;
  }
  const rest = { ...wire, ask_id: ask_id ?? wire.id, open_issue };
  if (isAnswerType(answer_type)) {
    const schemas: AnswerTypeSchemas = ANSWER_TYPE_SCHEMAS[answer_type];
    const parsedSettings = schemas.settings.safeParse(settings);
    if (parsedSettings.success) {
      // The settings schema fits AnswerSettingsByType of `answer_type`; TS
      // cannot correlate the lookup with the union.
      return { ...rest, answer_type, settings: parsedSettings.data } as QuestionRow;
    }
  }
  const unreadable: UnreadableQuestionRow = {
    ...rest,
    answer_type: "unknown",
    settings:
      typeof settings === "object" && settings !== null && !Array.isArray(settings)
        ? (settings as Record<string, unknown>)
        : {},
  };
  return unreadable;
}

export const QuestionRowSchema: z.ZodType<QuestionRow> = QuestionRowWireSchema.transform(readQuestionRow);

// 200 of `GET /api/questions`.
export const QuestionRowPageSchema = z.object({
  rows: listDroppingUnreadable(QuestionRowSchema),
  next_cursor: z.string().min(1).nullish().catch(null).transform((value) => value ?? null),
});

export const EMPTY_QUESTION_ROW_PAGE: QuestionRowPage = {
  rows: [],
  next_cursor: null,
};

// 200 of `GET /api/questions/open-count`.
export const QuestionOpenCountSchema = z.object({
  count: z.number().int().nonnegative().catch(0),
  blocking_count: z.number().int().nonnegative().catch(0),
});

export const EMPTY_QUESTION_OPEN_COUNT: QuestionOpenCount = {
  count: 0,
  blocking_count: 0,
};
