// @vitest-environment node
import { describe, expect, it } from "vitest";
import { projectSlugProblem } from "./slug";

const project = { id: "p-1", organisation_id: "org-1" };
const projects = [
  { id: "p-1", organisation_id: "org-1", slug: "multica" },
  { id: "p-2", organisation_id: "org-1", slug: "site" },
  { id: "p-3", organisation_id: "org-2", slug: "app" },
];

describe("projectSlugProblem", () => {
  it("accepts a folder name no other project of the organisation holds", () => {
    expect(projectSlugProblem("multica-web", project, projects, true)).toBeNull();
    expect(projectSlugProblem("multica", project, projects, true)).toBeNull();
    expect(projectSlugProblem("app", project, projects, true)).toBeNull();
  });

  it("requires a slug only while the project holds repositories", () => {
    expect(projectSlugProblem("", project, projects, true)).toBe("required");
    expect(projectSlugProblem("", project, projects, false)).toBeNull();
  });

  it.each(["Multica", "-multica", "multi_ca", "multi ca", "ä"])("rejects %s as a folder name", (slug) => {
    expect(projectSlugProblem(slug, project, projects, false)).toBe("format");
  });

  it("rejects the slug of another project of the same organisation", () => {
    expect(projectSlugProblem("site", project, projects, false)).toBe("taken");
  });

  it("finds no conflict for a project without an organisation", () => {
    expect(projectSlugProblem("site", { id: "p-4", organisation_id: null }, projects, false)).toBeNull();
  });
});
