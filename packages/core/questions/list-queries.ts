import { infiniteQueryOptions, queryOptions, useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { api } from "../api";
import type { QuestionRowFilters, QuestionTab } from "../types/question";
import { questionKeys, type QuestionParent } from "./queries";

export const EMPTY_QUESTION_ROW_FILTERS: QuestionRowFilters = {
  answer_types: [],
  asker_ids: [],
  answerer_ids: [],
  link_ids: [],
  blocks_work: false,
};

/** One cache entry per set of filters, whatever order their lists were picked in. */
function normalizedFilters(filters: QuestionRowFilters): QuestionRowFilters {
  return {
    answer_types: [...filters.answer_types].sort(),
    asker_ids: [...filters.asker_ids].sort(),
    answerer_ids: [...filters.answerer_ids].sort(),
    link_ids: [...filters.link_ids].sort(),
    blocks_work: filters.blocks_work,
  };
}

/** The rows of one tab of the Questions page, page by page over `next_cursor`. */
export function questionRowsOptions(wsId: string, tab: QuestionTab, filters: QuestionRowFilters) {
  const normalized = normalizedFilters(filters);
  return infiniteQueryOptions({
    queryKey: questionKeys.rows(wsId, tab, normalized),
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) => api.listQuestionRows(tab, normalized, pageParam, signal),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
    enabled: !!wsId,
  });
}

export function questionsOpenCountOptions(wsId: string) {
  return queryOptions({
    queryKey: questionKeys.openCount(wsId),
    queryFn: ({ signal }) => api.getOpenQuestionCount(signal),
    enabled: !!wsId,
  });
}

export function useQuestionRows(wsId: string, tab: QuestionTab, filters: QuestionRowFilters) {
  return useInfiniteQuery(questionRowsOptions(wsId, tab, filters));
}

/** The sidebar's Questions count: the questions open for the viewer. */
export function useQuestionsOpenCount(wsId: string): number {
  const { data } = useQuery(questionsOpenCountOptions(wsId));
  return data?.count ?? 0;
}

/** The first page of open rows below a parent issue or project, for its Questions property. */
export function questionParentRowsOptions(wsId: string, parent: QuestionParent) {
  return queryOptions({
    queryKey: questionKeys.parent(wsId, parent),
    queryFn: ({ signal }) =>
      api.listQuestionRows("all_open", { ...EMPTY_QUESTION_ROW_FILTERS, parent }, null, signal),
    enabled: !!wsId && !!parent.id,
  });
}

/** The open count below a parent issue or project, with how many block work. */
export function questionParentCountOptions(wsId: string, parent: QuestionParent) {
  return queryOptions({
    queryKey: questionKeys.parentCount(wsId, parent),
    queryFn: ({ signal }) => api.getOpenQuestionCount(signal, { tab: "all_open", parent }),
    enabled: !!wsId && !!parent.id,
  });
}
