"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Check, Search } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import type { PersonAnswerSettings, PersonAnswerValue } from "@multica/core/types";
import { agentListOptions, memberListOptions } from "@multica/core/workspace/queries";
import { Input } from "@multica/ui/components/ui/input";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../../common/actor-avatar";
import { matchesPinyin } from "../../editor/extensions/pinyin-match";
import { useT } from "../../i18n";
import type { AnswerControlProps } from "./control-props";
import type { AnswerableQuestion } from "./draft";

interface Person extends PersonAnswerValue {
  name: string;
}

/**
 * The members and agents the asker allows, unarchived; `undefined` while a
 * list they come from still loads or could not load.
 */
function useAllowedPeople(allow: PersonAnswerSettings["allow"], enabled = true): Person[] | undefined {
  const wsId = useWorkspaceId();
  const { data: members } = useQuery({ ...memberListOptions(wsId), enabled: enabled && allow !== "agents" });
  const { data: agents } = useQuery({ ...agentListOptions(wsId), enabled: enabled && allow !== "members" });
  if ((allow !== "agents" && !members) || (allow !== "members" && !agents)) return undefined;
  return [
    ...(allow === "agents" ? [] : (members ?? []).map((m) => ({ type: "member" as const, id: m.user_id, name: m.name }))),
    ...(allow === "members" ? [] : (agents ?? []).filter((a) => !a.archived_at).map((a) => ({ type: "agent" as const, id: a.id, name: a.name }))),
  ];
}

/**
 * False while a suggested Person is not on screen by name: its lists still
 * load, or it is not among the people the asker allows. Accept suggestion
 * waits for it; every other suggestion shows from the question alone.
 */
export function useSuggestionShown(question: AnswerableQuestion): boolean {
  const suggested = question.answer_type === "person" ? question.suggested_answer : null;
  const people = useAllowedPeople(question.answer_type === "person" ? question.settings.allow : "both", suggested !== null);
  return suggested === null || people?.some((person) => person.type === suggested.type && person.id === suggested.id) === true;
}

/**
 * A search input and the members and agents the asker allows, one row each.
 * The person picked or suggested keeps its row on top whatever the search.
 */
export function PersonControl({ question, draft, onDraftChange, disabled }: AnswerControlProps<"person">) {
  const { t } = useT("questions");
  const people = useAllowedPeople(question.settings.allow) ?? [];
  const [search, setSearch] = useState("");
  const query = search.trim().toLowerCase();
  const matches = (name: string) => !query || name.toLowerCase().includes(query) || matchesPinyin(name, query);

  const shown = draft ? people.find((person) => person.type === draft.type && person.id === draft.id) : undefined;
  const rows = [
    ...(shown ? [shown] : []),
    ...people.filter((person) => person !== shown && matches(person.name)),
  ];

  return (
    <div className="flex flex-col gap-1">
      <div className="relative">
        <Search aria-hidden className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={search}
          disabled={disabled}
          aria-label={t(($) => $.controls.person_search)}
          placeholder={t(($) => $.controls.person_search)}
          className="pl-8"
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>
      <div role="listbox" aria-label={t(($) => $.controls.person_search)} className="flex max-h-56 flex-col overflow-y-auto">
        {rows.map((person) => {
          const selected = person === shown;
          return (
            <button
              key={`${person.type}:${person.id}`}
              type="button"
              role="option"
              aria-selected={selected}
              disabled={disabled}
              onClick={() => onDraftChange({ type: person.type, id: person.id })}
              className={cn(
                "flex h-9 items-center gap-2 rounded-md px-2 text-left text-body outline-none",
                "hover:bg-muted/50 focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50",
                selected && "bg-muted",
              )}
            >
              <ActorAvatar actorType={person.type} actorId={person.id} size="sm" />
              <span className="truncate">{person.name}</span>
              <span className="text-caption text-muted-foreground">
                {person.type === "member" ? t(($) => $.controls.person_member) : t(($) => $.controls.person_agent)}
              </span>
              {selected && <Check aria-hidden className="ml-auto size-4" />}
            </button>
          );
        })}
        {rows.length === 0 && (
          <p className="px-2 py-1.5 text-caption text-muted-foreground">{t(($) => $.controls.person_none)}</p>
        )}
      </div>
    </div>
  );
}
