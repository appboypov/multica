import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { defaultStorage } from "../platform/storage";
import type { QuestionRowFilters } from "../types/question";
import { EMPTY_QUESTION_ROW_FILTERS } from "./list-queries";
import type { QuestionGrouping } from "./rows";

export type AnswerAllView = "one" | "page";

/** The filter dimensions that hold a list of picked values. */
export type QuestionListFilter = "answer_types" | "asker_ids" | "answerer_ids" | "link_ids";

interface QuestionsViewState {
  /** In memory per workspace: a filter is picked for one sitting, not kept. */
  filtersByWorkspace: Record<string, QuestionRowFilters>;
  grouping: QuestionGrouping;
  /** The Answer all view each viewer used last on this device, by user id. */
  answerAllView: Record<string, AnswerAllView>;
  toggleFilter: (wsId: string, dimension: QuestionListFilter, value: string) => void;
  toggleBlocking: (wsId: string) => void;
  clearFilters: (wsId: string) => void;
  setGrouping: (grouping: QuestionGrouping) => void;
  setAnswerAllView: (userId: string, view: AnswerAllView) => void;
}

const GROUPINGS: readonly QuestionGrouping[] = ["link", "due", "asker"];

export const useQuestionsViewStore = create<QuestionsViewState>()(
  persist(
    (set) => ({
      filtersByWorkspace: {},
      grouping: "link",
      answerAllView: {},
      toggleFilter: (wsId, dimension, value) =>
        set((state) => {
          const current = state.filtersByWorkspace[wsId] ?? EMPTY_QUESTION_ROW_FILTERS;
          const values: readonly string[] = current[dimension];
          const next = values.includes(value) ? values.filter((v) => v !== value) : [...values, value];
          return { filtersByWorkspace: { ...state.filtersByWorkspace, [wsId]: { ...current, [dimension]: next } } };
        }),
      toggleBlocking: (wsId) =>
        set((state) => {
          const current = state.filtersByWorkspace[wsId] ?? EMPTY_QUESTION_ROW_FILTERS;
          return {
            filtersByWorkspace: { ...state.filtersByWorkspace, [wsId]: { ...current, blocks_work: !current.blocks_work } },
          };
        }),
      clearFilters: (wsId) =>
        set((state) => {
          const { [wsId]: _cleared, ...rest } = state.filtersByWorkspace;
          return { filtersByWorkspace: rest };
        }),
      setGrouping: (grouping) => set({ grouping }),
      setAnswerAllView: (userId, view) =>
        set((state) => ({ answerAllView: { ...state.answerAllView, [userId]: view } })),
    }),
    {
      name: "multica_questions_view",
      storage: createJSONStorage(() => defaultStorage),
      partialize: ({ grouping, answerAllView }) => ({ grouping, answerAllView }),
      merge: (persisted, current) => {
        const saved = (persisted ?? {}) as Partial<Pick<QuestionsViewState, "grouping" | "answerAllView">>;
        const answerAllView: Record<string, AnswerAllView> = {};
        for (const [userId, view] of Object.entries(saved.answerAllView ?? {})) {
          if (view === "one" || view === "page") answerAllView[userId] = view;
        }
        return {
          ...current,
          grouping: saved.grouping && GROUPINGS.includes(saved.grouping) ? saved.grouping : current.grouping,
          answerAllView,
        };
      },
    },
  ),
);

/** The workspace's Questions page filters; a stable reference while unchanged. */
export function useQuestionFilters(wsId: string): QuestionRowFilters {
  return useQuestionsViewStore((state) => state.filtersByWorkspace[wsId] ?? EMPTY_QUESTION_ROW_FILTERS);
}

/** The number of picked filter values, as the filter button counts them. */
export function questionFilterCount(filters: QuestionRowFilters): number {
  return filters.answer_types.length + filters.asker_ids.length + filters.answerer_ids.length +
    filters.link_ids.length + (filters.blocks_work ? 1 : 0);
}
