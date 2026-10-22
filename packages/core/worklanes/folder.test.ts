// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  DEFAULT_WORK_ROOT,
  checkoutPath,
  folderFromName,
  folderProblem,
  workspaceWorkRoot,
  worklanePath,
} from "./folder";

const place = { workRoot: "~/hq", organisationKey: "das", projectSlug: "multica" };

describe("workspaceWorkRoot", () => {
  it.each([
    ["~/work", "~/work"],
    ["~", "~"],
    ["/srv/work", "/srv/work"],
    [" ~/work// ", "~/work"],
    ["/srv//work/./", "/srv/work"],
    ["~/a..b", "~/a..b"],
  ])("reads work_root %j as %j", (workRoot, expected) => {
    expect(workspaceWorkRoot({ settings: { work_root: workRoot } })).toBe(expected);
  });

  it.each([
    ["no workspace", null],
    ["no work_root", { settings: {} }],
    ["an empty work_root", { settings: { work_root: "  " } }],
    ["a relative work_root", { settings: { work_root: "work" } }],
    ["a work_root under another home", { settings: { work_root: "~brian/work" } }],
    ["a work_root with ..", { settings: { work_root: "~/work/../etc" } }],
    ["a malformed work_root", { settings: { work_root: 42 } as unknown as { work_root: string } }],
  ])("defaults to ~/hq for %s", (_name, workspace) => {
    expect(workspaceWorkRoot(workspace)).toBe("~/hq");
    expect(DEFAULT_WORK_ROOT).toBe("~/hq");
  });
});

describe("checkoutPath", () => {
  it("places the checkout under the project's repos folder by repository name", () => {
    expect(checkoutPath(place, "appboypov/multica")).toBe("~/hq/das/multica/repos/multica");
  });

  it("follows the work root", () => {
    expect(checkoutPath({ ...place, workRoot: "/srv/work" }, "appboypov/multica-ui")).toBe(
      "/srv/work/das/multica/repos/multica-ui",
    );
  });

  it.each([
    ["organisation", { ...place, organisationKey: null }],
    ["slug", { ...place, projectSlug: null }],
    ["empty slug", { ...place, projectSlug: "" }],
  ])("has no path without the project's %s", (_name, missing) => {
    expect(checkoutPath(missing, "appboypov/multica")).toBeNull();
  });
});

describe("worklanePath", () => {
  it("places the worktree under the project's worktrees folder by repository and folder", () => {
    expect(worklanePath(place, "appboypov/multica", "hq-paths")).toBe(
      "~/hq/das/multica/worktrees/multica/hq-paths",
    );
  });

  it("uses only the name of owner/name", () => {
    expect(worklanePath({ ...place, projectSlug: "site" }, "rebl/rebl-web", "copy")).toBe(
      "~/hq/das/site/worktrees/rebl-web/copy",
    );
  });

  it.each([
    ["organisation", { ...place, organisationKey: undefined }],
    ["slug", { ...place, projectSlug: null }],
  ])("has no path without the project's %s", (_name, missing) => {
    expect(worklanePath(missing, "appboypov/multica", "hq-paths")).toBeNull();
  });
});

describe("folderFromName", () => {
  it("lowercases and dashes a name", () => {
    expect(folderFromName("  HQ Paths!  v2 ")).toBe("hq-paths-v2");
  });
});

describe("folderProblem", () => {
  it("flags an invalid folder and one a worklane holds", () => {
    const worklanes = [{ name: "HQ paths", folder: "hq-paths" }];
    expect(folderProblem("", worklanes)).toEqual({ kind: "invalid" });
    expect(folderProblem("-x", worklanes)).toEqual({ kind: "invalid" });
    expect(folderProblem("hq-paths", worklanes)).toEqual({ kind: "in_use", worklaneName: "HQ paths" });
    expect(folderProblem("other", worklanes)).toBeNull();
  });
});
