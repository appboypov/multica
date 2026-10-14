"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { projectListOptions } from "@multica/core/projects";
import { repositoryListOptions } from "@multica/core/repositories";
import { countWorklanes, worklaneListOptions, type WorklaneCounts } from "@multica/core/worklanes";

/**
 * The number of worklanes, `main` included, of every repository, project and
 * organisation of the workspace, from the cached lists, so a created or
 * removed worklane updates every count at once.
 */
export function useWorklaneCounts(): WorklaneCounts {
  const wsId = useWorkspaceId();
  const { data: worklanes } = useQuery(worklaneListOptions(wsId));
  const { data: repositories } = useQuery(repositoryListOptions(wsId));
  const { data: projects } = useQuery(projectListOptions(wsId));
  return useMemo(
    () => countWorklanes(worklanes ?? [], repositories ?? [], projects ?? []),
    [worklanes, repositories, projects],
  );
}
