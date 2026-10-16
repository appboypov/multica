"use client";

import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { clientErrorMessage } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  releaseActivityOptions,
  releaseCommentsOptions,
  useCreateReleaseComment,
} from "@multica/core/releases";
import type { ReleaseComment, ReleaseOwnerActivityEntry } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Spinner } from "@multica/ui/components/ui/spinner";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { ActorAvatar } from "../../common/actor-avatar";
import { ReadonlyContent } from "../../editor/readonly-content";
import { useLocale, useT } from "../../i18n";
import { ownerChangeMessage } from "./release-messages";

const NO_COMMENTS: ReleaseComment[] = [];
const NO_ENTRIES: ReleaseOwnerActivityEntry[] = [];

type ThreadItem =
  | { kind: "comment"; id: string; at: string; comment: ReleaseComment }
  | { kind: "activity"; id: string; at: string; entry: ReleaseOwnerActivityEntry };

/** Oldest first; equal times keep a stable order by id. */
function byTime(a: { at: string; id: string }, b: { at: string; id: string }): number {
  const diff = Date.parse(a.at) - Date.parse(b.at);
  return diff !== 0 ? diff : a.id.localeCompare(b.id);
}

/**
 * A release page's discussion: root comments and owner changes in time
 * order, each comment's replies under it, oldest first, then the comment
 * box. A reply whose root is outside the loaded window shows as a root so
 * it is never hidden.
 */
export function ReleaseComments({ releaseId }: { releaseId: string }) {
  const wsId = useWorkspaceId();
  const { t } = useT("releases");
  const headingId = useId();
  const comments = useQuery(releaseCommentsOptions(wsId, releaseId));
  const activity = useQuery(releaseActivityOptions(wsId, releaseId));
  const [replyingTo, setReplyingTo] = useState<string | null>(null);

  const all = comments.data ?? NO_COMMENTS;
  const byId = new Map(all.map((comment) => [comment.id, comment]));
  const replies = new Map<string, ThreadItem[]>();
  const items: ThreadItem[] = (activity.data ?? NO_ENTRIES).map((entry) => ({
    kind: "activity",
    id: entry.id,
    at: entry.created_at,
    entry,
  }));
  for (const comment of all) {
    const item: ThreadItem = { kind: "comment", id: comment.id, at: comment.created_at, comment };
    if (comment.parent_id && byId.has(comment.parent_id)) {
      let root = byId.get(comment.parent_id)!;
      while (root.parent_id && byId.has(root.parent_id)) {
        root = byId.get(root.parent_id)!;
      }
      const thread = replies.get(root.id) ?? [];
      thread.push(item);
      replies.set(root.id, thread);
    } else {
      items.push(item);
    }
  }
  items.sort(byTime);

  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-4">
      <h2 id={headingId} className="text-body-lg font-medium">
        {t(($) => $.comments.title)}
      </h2>
      {comments.isLoading || activity.isLoading ? (
        <div className="flex flex-col gap-3">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : items.length === 0 ? (
        <p className="text-label text-muted-foreground">{t(($) => $.comments.empty)}</p>
      ) : (
        <ol className="flex flex-col gap-4">
          {items.map((item) =>
            item.kind === "activity" ? (
              <li key={`activity:${item.id}`}>
                <OwnerChange entry={item.entry} />
              </li>
            ) : (
              <li key={`comment:${item.id}`} className="flex flex-col gap-3">
                <CommentView comment={item.comment} onReply={() => setReplyingTo(item.id)} />
                {(replies.get(item.id) ?? []).length > 0 || replyingTo === item.id ? (
                  <ol className="ml-8 flex flex-col gap-3 border-l border-border pl-4">
                    {(replies.get(item.id) ?? []).sort(byTime).map((reply) =>
                      reply.kind === "comment" ? (
                        <li key={`comment:${reply.id}`}>
                          <CommentView comment={reply.comment} />
                        </li>
                      ) : null,
                    )}
                    {replyingTo === item.id ? (
                      <li>
                        <CommentComposer
                          releaseId={releaseId}
                          parentId={item.id}
                          onSent={() => setReplyingTo(null)}
                          onCancel={() => setReplyingTo(null)}
                        />
                      </li>
                    ) : null}
                  </ol>
                ) : null}
              </li>
            ),
          )}
        </ol>
      )}
      <CommentComposer releaseId={releaseId} />
    </section>
  );
}

function When({ at }: { at: string }) {
  const locale = useLocale();
  return (
    <time dateTime={at} className="text-caption text-muted-foreground">
      {new Date(at).toLocaleString(locale, { dateStyle: "long", timeStyle: "short" })}
    </time>
  );
}

function OwnerChange({ entry }: { entry: ReleaseOwnerActivityEntry }) {
  const { t } = useT("releases");
  const { getActorName } = useActorName();
  const actorId = entry.actor_id ?? "";
  const actor = getActorName(entry.actor_type, actorId);
  const message = ownerChangeMessage(t, actor, entry.details, getActorName);
  if (message === null) return null;
  return (
    <div className="flex items-center gap-2 text-label text-muted-foreground">
      <ActorAvatar actorType={entry.actor_type} actorId={actorId} name={actor} size="sm" />
      <span className="min-w-0 flex-1">{message}</span>
      <When at={entry.created_at} />
    </div>
  );
}

function CommentView({ comment, onReply }: { comment: ReleaseComment; onReply?: () => void }) {
  const { t } = useT("releases");
  const { getActorName } = useActorName();
  const author = getActorName(comment.author_type, comment.author_id);
  return (
    <article aria-label={author} className="flex gap-3">
      <ActorAvatar actorType={comment.author_type} actorId={comment.author_id} name={author} size="sm" />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex items-center gap-2">
          <span className="text-label font-medium">{author}</span>
          <When at={comment.created_at} />
        </div>
        <ReadonlyContent content={comment.content} attachments={comment.attachments} className="text-body" />
        {onReply ? (
          <div>
            <Button type="button" variant="ghost" size="sm" onClick={onReply}>
              {t(($) => $.comments.reply)}
            </Button>
          </div>
        ) : null}
      </div>
    </article>
  );
}

/**
 * The comment box, or a reply box under a root comment. The text stays
 * until the server took it, so a failed send shows why and can be retried.
 */
function CommentComposer({
  releaseId,
  parentId,
  onSent,
  onCancel,
}: {
  releaseId: string;
  parentId?: string;
  onSent?: () => void;
  onCancel?: () => void;
}) {
  const { t } = useT("releases");
  const create = useCreateReleaseComment(releaseId);
  const [content, setContent] = useState("");
  const [failure, setFailure] = useState<string | null>(null);
  const label = parentId ? t(($) => $.comments.reply_placeholder) : t(($) => $.comments.placeholder);
  const pending = create.isPending;
  const empty = content.trim() === "";

  const send = async () => {
    if (empty || pending) return;
    setFailure(null);
    try {
      await create.mutateAsync(parentId ? { content, parent_id: parentId } : { content });
      setContent("");
      onSent?.();
    } catch (err) {
      setFailure(clientErrorMessage(err) ?? t(($) => $.comments.failed));
    }
  };

  return (
    <form
      className="flex flex-col gap-2"
      onSubmit={(event) => {
        event.preventDefault();
        void send();
      }}
    >
      <Textarea
        aria-label={label}
        placeholder={label}
        value={content}
        onChange={(event) => setContent(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
            event.preventDefault();
            void send();
          }
        }}
        autoFocus={parentId !== undefined}
        disabled={pending}
        className="min-h-20 text-body"
      />
      {failure !== null ? (
        <div role="alert" className="flex items-center gap-2 text-label text-destructive">
          <span className="min-w-0 flex-1">{failure}</span>
          <Button type="button" variant="outline" size="sm" onClick={() => void send()} disabled={pending}>
            {t(($) => $.comments.retry)}
          </Button>
        </div>
      ) : null}
      <div className="flex justify-end gap-2">
        {onCancel ? (
          <Button type="button" variant="ghost" size="sm" onClick={onCancel} disabled={pending}>
            {t(($) => $.comments.cancel)}
          </Button>
        ) : null}
        <Button type="submit" size="sm" disabled={empty || pending} aria-busy={pending}>
          {pending ? <Spinner /> : null}
          {parentId ? t(($) => $.comments.reply) : t(($) => $.comments.send)}
        </Button>
      </div>
    </form>
  );
}
