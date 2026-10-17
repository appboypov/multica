// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  SELECTION_BLOCK,
  appendSelection,
  insertSelectionPlaceholder,
  selectionWarning,
} from "./instruction-selection";

describe("appendSelection", () => {
  it("keeps existing order and appends each new path once in added order", () => {
    const current = ["/notes/b.md", "/skills/reviewer"];
    const added = ["/skills/reviewer", "/notes/c.md", "/skills/planner", "/notes/c.md"];

    expect(appendSelection(current, added)).toEqual([
      "/notes/b.md",
      "/skills/reviewer",
      "/notes/c.md",
      "/skills/planner",
    ]);
    expect(current).toEqual(["/notes/b.md", "/skills/reviewer"]);
    expect(added).toHaveLength(4);
  });

  it("preserves exact paths without normalizing or sorting them", () => {
    expect(appendSelection([], ["/skills/B", "/skills/a", "/skills/B"])).toEqual([
      "/skills/B",
      "/skills/a",
    ]);
  });
});

describe("insertSelectionPlaceholder", () => {
  it.each([0, 7, 12])("inserts at cursor %i and places the cursor after the placeholder", (cursor) => {
    const text = "Follow this.";
    expect(insertSelectionPlaceholder(text, cursor)).toEqual({
      text: text.slice(0, cursor) + SELECTION_BLOCK + text.slice(cursor),
      cursor: cursor + SELECTION_BLOCK.length,
    });
  });

  it("appends after a blank line when there is no cursor", () => {
    expect(insertSelectionPlaceholder("Follow this.", null)).toEqual({
      text: "Follow this.\n\n<SystemIdentity>\n{selection}\n</SystemIdentity>",
      cursor: "Follow this.\n\n<SystemIdentity>\n{selection}\n</SystemIdentity>".length,
    });
  });

  it.each([null, 0])("inserts alone in empty text with cursor %s", (cursor) => {
    expect(insertSelectionPlaceholder("", cursor)).toEqual({
      text: SELECTION_BLOCK,
      cursor: SELECTION_BLOCK.length,
    });
  });
});

describe("selectionWarning", () => {
  it("warns that selected paths will not load without the placeholder", () => {
    expect(selectionWarning("Follow this.", ["/skills/reviewer"])).toBe("not_loaded");
  });

  it("warns that a placeholder has no selection to load", () => {
    expect(selectionWarning("Follow this. {selection}", [])).toBe("empty_placeholder");
  });

  it("does not warn when a selection and placeholder are both present", () => {
    expect(selectionWarning("{selection}\n{selection}", ["/skills/reviewer"])).toBeNull();
  });

  it("does not warn when neither a selection nor placeholder is present", () => {
    expect(selectionWarning("Follow this.", [])).toBeNull();
  });
});
