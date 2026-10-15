// @vitest-environment node
import { describe, expect, it } from "vitest";
import { EMPTY_RELEASE, ReleaseLineResponseSchema } from "./schemas";
import { toLine, lineStates, nextRelease, previewEdit, previewMove } from "../releases/line";

const first = { ...EMPTY_RELEASE, id: "first", repository_id: "repo", position: 0, version: "1.1.0", bump: "minor" as const, title: "Published", shipped_at: "2026-10-15T12:00:00Z" };
const next = { ...EMPTY_RELEASE, id: "next", repository_id: "repo", position: 1, version: "1.1.1", bump: "patch" as const, title: "Next" };

describe("manual release shipping", () => {
  it("preserves the ship date in the line response and advances next", () => {
    const answer = ReleaseLineResponseSchema.parse({ releases: [first, next] });
    expect(answer.releases[0]?.shipped_at).toBe(first.shipped_at);
    const line = toLine(answer.releases, "1.0.0");
    expect(lineStates(line).get(first.id)).toBe("shipped");
    expect(lineStates(line).get(next.id)).toBe("next");
    expect(nextRelease(line)?.id).toBe(next.id);
  });

  it("keeps a shipped version fixed and prevents reordering across it", () => {
    const { releases } = ReleaseLineResponseSchema.parse({ releases: [first, next] });
    const line = toLine(releases, "1.0.0");
    expect(previewEdit(line, first.id, { title: first.title, version: first.version, bump: "major" }).problem?.code).toBe("release_shipped");
    expect(previewEdit(line, first.id, { title: "Renamed", version: first.version, bump: first.bump }).problem).toBeNull();
    expect(previewMove(line, first.id, "later").problem?.code).toBe("release_shipped");
    expect(previewMove(line, next.id, "earlier").problem?.code).toBe("release_before_shipped");
  });
});
