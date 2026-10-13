import { toggleOption, type AnswerDraft, type AnswerDraftByType, type AnswerableQuestion } from "./draft";

/**
 * The draft a focus key of the UX spec picks while focus is in the card: `Y`
 * and `N` for Yes / No, `1`-`9` for the options of Single and Multiple choice
 * (Single choice's Other comes after its options). `undefined` when the key
 * picks nothing, so the event is left alone. Ranking moves its focused item
 * itself and Text sends on ⌘↵ in its editor.
 */
export function draftForKey(question: AnswerableQuestion, draft: AnswerDraft, key: string): AnswerDraft | undefined {
  switch (question.answer_type) {
    case "yes_no": {
      const choice = key.toLowerCase() === "y" ? "yes" : key.toLowerCase() === "n" ? "no" : null;
      return choice === null ? undefined : { choice };
    }
    case "single_choice": {
      const index = Number(key) - 1;
      if (!/^[1-9]$/.test(key)) return undefined;
      const option = question.settings.options[index];
      if (option !== undefined) return { option };
      if (question.settings.allow_other && index === question.settings.options.length) {
        const current = draft as AnswerDraftByType["single_choice"] | null;
        return { other: current && "other" in current ? current.other : "" };
      }
      return undefined;
    }
    case "multiple_choice": {
      if (!/^[1-9]$/.test(key)) return undefined;
      const option = question.settings.options[Number(key) - 1];
      return option === undefined ? undefined : toggleOption(question, draft as AnswerDraftByType["multiple_choice"] | null, option);
    }
    default:
      return undefined;
  }
}
