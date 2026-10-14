import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";

export const agentQuestionsKeys = {
  settings: (wsId: string) => ["agent-questions", wsId] as const,
};

/** The workspace Questions default, read by the settings tab and every issue's Agent questions row. */
export function agentQuestionsOptions(wsId: string) {
  return queryOptions({
    queryKey: agentQuestionsKeys.settings(wsId),
    queryFn: () => api.getAgentQuestions(),
    enabled: !!wsId,
  });
}

/** Sets the workspace Questions default; awaits the server and stores its response. */
export function useUpdateAgentQuestionsDefault(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) => api.updateAgentQuestions({ enabled }),
    onSuccess: (settings) => qc.setQueryData(agentQuestionsKeys.settings(wsId), settings),
  });
}
