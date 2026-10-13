// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { Organisation, Project } from "../types";
import { EMPTY_PROJECT } from "../api/schemas";
import { groupProjectsByOrganisation } from "./group";

const das: Organisation = { id: "das", workspace_id: "ws", name: "de App Specialist", key: "das", created_at: "", updated_at: "" };
const rebl: Organisation = { ...das, id: "rebl", name: "Rebl Records", key: "rr" };
const empty: Organisation = { ...das, id: "empty", name: "Buttler", key: "btlr" };
const multica: Project = { ...EMPTY_PROJECT, id: "multica", title: "Multica", organisation_id: das.id };
const fan: Project = { ...EMPTY_PROJECT, id: "fan", title: "Fan app", organisation_id: rebl.id };
const reblProject: Project = { ...fan, id: "rebl-project", title: "Rebl" };
const unassigned: Project = { ...EMPTY_PROJECT, id: "overview", title: "das-overview" };

describe("groupProjectsByOrganisation", () => {
  it("orders organisations by name ignoring case, omits empty groups and puts no organisation last", () => {
    const groups = groupProjectsByOrganisation([unassigned, fan, multica], [rebl, empty, das]);
    expect(groups.map((group) => group.organisation?.id ?? null)).toEqual([das.id, rebl.id, null]);
    expect(groups.map((group) => group.projects)).toEqual([[multica], [fan], [unassigned]]);
  });

  it("keeps the input order inside each group without mutating either input", () => {
    const projects = Object.freeze([fan, multica, reblProject]);
    const organisations = Object.freeze([rebl, das]);
    expect(groupProjectsByOrganisation(projects, organisations)[1]?.projects).toEqual([fan, reblProject]);
    expect(projects).toEqual([fan, multica, reblProject]);
    expect(organisations).toEqual([rebl, das]);
  });

  it("omits no organisation when every project belongs to a known organisation", () => {
    expect(groupProjectsByOrganisation([fan], [rebl])).toEqual([{ organisation: rebl, projects: [fan] }]);
    expect(groupProjectsByOrganisation([], [das, rebl])).toEqual([]);
  });

  it("groups an unknown organisation with the unassigned projects in input order", () => {
    const unknown = { ...fan, id: "unknown", organisation_id: "missing" };
    expect(groupProjectsByOrganisation([unknown, multica, unassigned], [das])).toEqual([
      { organisation: das, projects: [multica] },
      { organisation: null, projects: [unknown, unassigned] },
    ]);
  });
});
