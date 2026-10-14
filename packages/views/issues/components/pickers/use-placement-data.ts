"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Organisation, Project, Release, Repository, Worklane } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { worklaneListOptions } from "@multica/core/worklanes";
import { releaseListOptions } from "@multica/core/releases";
import { repositoryListOptions } from "@multica/core/repositories";
import { organisationListOptions } from "@multica/core/organisations";
import { projectListOptions } from "@multica/core/projects";
import type { PlacementData } from "@multica/core/issues/placement";

const NO_WORKLANES: Worklane[] = [];
const NO_RELEASES: Release[] = [];
const NO_REPOSITORIES: Repository[] = [];
const NO_ORGANISATIONS: Organisation[] = [];
const NO_PROJECTS: Project[] = [];

export interface PlacementReads extends PlacementData {
  organisations: readonly Organisation[];
  projects: readonly Project[];
}

/**
 * The cached worklanes, repositories, releases, projects and organisations the
 * Worklane and Release pickers and New issue's defaults read.
 */
export function usePlacementData(): PlacementReads {
  const wsId = useWorkspaceId();
  const { data: worklanes = NO_WORKLANES } = useQuery(worklaneListOptions(wsId));
  const { data: releases = NO_RELEASES } = useQuery(releaseListOptions(wsId));
  const { data: repositories = NO_REPOSITORIES } = useQuery(repositoryListOptions(wsId));
  const { data: organisations = NO_ORGANISATIONS } = useQuery(organisationListOptions(wsId));
  const { data: projects = NO_PROJECTS } = useQuery(projectListOptions(wsId));
  return useMemo(
    () => ({ worklanes, releases, repositories, organisations, projects }),
    [worklanes, releases, repositories, organisations, projects],
  );
}
