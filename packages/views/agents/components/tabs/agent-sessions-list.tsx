"use client";

import { Terminal } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import type { Agent, AgentSession } from "@multica/core/types";
import { agentSessionsOptions } from "@multica/core/agents";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../../../navigation";
import { useT, useTimeAgo } from "../../../i18n";
import { HOME_PREFIX } from "../instructions-field";

/**
 * The agent's active sessions: sessions started by hand and runs, one row
 * each. Shows nothing while the first load runs; a failed load already
 * falls back to an empty list and so shows the empty text.
 */
export function AgentSessionsList({ agent }: { agent: Agent }) {
  const wsId = useWorkspaceId();
  const { t } = useT("agents");
  const { data, isPending } = useQuery(agentSessionsOptions(wsId, agent.id));

  if (isPending) return null;
  const sessions = data?.sessions ?? [];
  if (sessions.length === 0) {
    return (
      <p className="text-caption italic text-muted-foreground">
        {t(($) => $.tab_body.activity.empty_sessions)}
      </p>
    );
  }
  return (
    <ul className="flex flex-col gap-2">
      {sessions.map((session) => (
        <SessionRow key={session.id} session={session} />
      ))}
    </ul>
  );
}

function SessionRow({ session }: { session: AgentSession }) {
  const { t } = useT("agents");
  const timeAgo = useTimeAgo();
  const paths = useWorkspacePaths();
  const folder = session.work_dir.replace(HOME_PREFIX, "~");
  const details = [session.repository, session.branch].filter(
    (value): value is string => !!value,
  );

  return (
    <li
      className="flex min-w-0 flex-col gap-0.5 rounded-md border px-3 py-2"
      data-testid={`session-${session.id}`}
    >
      <div className="flex min-w-0 items-center gap-2 text-body">
        <SessionWhere session={session} />
        {session.issue && (
          <AppLink
            href={paths.issueDetail(session.issue.id)}
            className="shrink-0 font-mono text-caption text-muted-foreground hover:text-foreground hover:underline"
          >
            {session.issue.identifier}
          </AppLink>
        )}
      </div>
      {folder && (
        // The end of a path names the work, so the start is what gets cut.
        <span
          dir="rtl"
          title={session.work_dir}
          className="truncate text-left font-mono text-caption text-muted-foreground"
        >
          <bdi dir="ltr">{folder}</bdi>
        </span>
      )}
      <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-caption text-muted-foreground">
        {details.map((value) => (
          <span key={value} className="flex items-center gap-2">
            <span className="font-mono">{value}</span>
            <span className="text-faint-foreground">·</span>
          </span>
        ))}
        <span>{t(($) => $.tab_body.activity.session_status[session.status])}</span>
        <span className="text-faint-foreground">·</span>
        <span>{timeAgo(session.started_at)}</span>
      </div>
    </li>
  );
}

function SessionWhere({ session }: { session: AgentSession }) {
  const { t } = useT("agents");
  if (session.kind === "terminal" && session.herdr_pane_id) {
    return (
      <span className="flex shrink-0 items-center gap-1.5 font-mono">
        <Terminal className="h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />
        {session.herdr_pane_id}
      </span>
    );
  }
  if (session.kind === "run" && session.runtime) {
    return (
      <span className="truncate">
        {t(($) => $.tab_body.activity.session_run_on, { runtime: session.runtime!.name })}
      </span>
    );
  }
  return (
    <span className="flex shrink-0 items-center gap-1.5">
      {session.kind === "terminal" && (
        <Terminal className="h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />
      )}
      {t(($) => $.tab_body.activity.session_unknown_where)}
    </span>
  );
}
