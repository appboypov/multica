import { queryOptions } from "@tanstack/react-query";
import { api } from "../../api";

/** The eleven templates are the server's data and the same in every workspace, so the key is account-level. */
export const issueTemplateKeys = {
  all: ["issue-templates"] as const,
};

export function issueTemplatesOptions() {
  return queryOptions({
    queryKey: issueTemplateKeys.all,
    queryFn: () => api.listIssueTemplates(),
    staleTime: Infinity,
  });
}
