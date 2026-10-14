"use client";

import { useWorkspaceId } from "@multica/core/hooks";
import { useActorName } from "@multica/core/workspace/hooks";
import { useWorklaneActivity } from "@multica/core/worklanes";
import type { Worklane, WorklaneActivityEntry } from "@multica/core/types";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { ActorAvatar } from "../../common/actor-avatar";
import { useLocale, useT } from "../../i18n";

/**
 * The worklane's activity in the order it happened: who did it, what
 * happened and when. Entries of the worklane and repository-wide entries of
 * its repository come from the server together (ADR-0008); an action this
 * view has no sentence for is not rendered.
 */
export function WorklaneActivity({ worklane }: { worklane: Worklane }) {
  const { t } = useT("worklanes");
  const wsId = useWorkspaceId();
  const { data: entries, isLoading } = useWorklaneActivity(wsId, worklane.id);

  return (
    <section aria-labelledby="worklane-activity-title" className="space-y-3">
      <h2 id="worklane-activity-title" className="text-body font-medium text-foreground">
        {t(($) => $.activity.title)}
      </h2>
      {isLoading ? (
        <div className="space-y-3">
          <Skeleton className="h-5 w-72" />
          <Skeleton className="h-5 w-60" />
        </div>
      ) : !entries || entries.length === 0 ? (
        <p className="text-body text-muted-foreground">{t(($) => $.activity.empty)}</p>
      ) : (
        <ol className="space-y-3">
          {entries.map((entry) => (
            <WorklaneActivityRow key={entry.id} entry={entry} />
          ))}
        </ol>
      )}
    </section>
  );
}

function WorklaneActivityRow({ entry }: { entry: WorklaneActivityEntry }) {
  const { t } = useT("worklanes");
  const locale = useLocale();
  const { getActorName } = useActorName();
  const actorId = entry.actor_id ?? "";
  const actor = getActorName(entry.actor_type, actorId);

  let sentence: string;
  switch (entry.action) {
    case "worklane_created":
      sentence = t(($) => $.activity.worklane_created, {
        actor,
        name: entry.details.name,
        runtime: entry.details.runtime_name,
      });
      break;
    case "worklane_renamed":
      sentence = t(($) => $.activity.worklane_renamed, {
        actor,
        from: entry.details.from,
        to: entry.details.to,
      });
      break;
    case "worklane_switched":
      sentence = t(($) => $.branch.switched, {
        from: entry.details.from || t(($) => $.branch.detached),
        to: entry.details.to || t(($) => $.branch.detached),
      });
      break;
    case "worklane_kept":
      sentence =
        entry.details.reason === "uncommitted"
          ? t(($) => $.branch.kept_uncommitted, { branch: entry.details.branch })
          : entry.details.reason === "unpushed"
            ? t(($) => $.branch.kept_unpushed, { branch: entry.details.branch, count: entry.details.count })
            : t(($) => $.branch.kept, { branch: entry.details.branch });
      break;
    default:
      return null;
  }

  return (
    <li className="flex items-start gap-2.5">
      <ActorAvatar actorType={entry.actor_type} actorId={actorId} name={actor} size="sm" />
      <div className="flex min-w-0 flex-1 flex-wrap items-baseline gap-x-2">
        <span className="text-body text-foreground">{sentence}</span>
        <time dateTime={entry.created_at} className="text-caption text-muted-foreground">
          {new Date(entry.created_at).toLocaleString(locale, { dateStyle: "long", timeStyle: "short" })}
        </time>
      </div>
    </li>
  );
}
