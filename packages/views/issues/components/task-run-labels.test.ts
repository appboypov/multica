// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { AgentTask } from "@multica/core/types";
import { plainTriggerSummary, runNumbersByTask } from "./task-run-labels";

const IMAGE = "[Image]";

describe("plainTriggerSummary", () => {
  it("decodes the entities the editor escapes", () => {
    expect(plainTriggerSummary("@Lambda fix conflicts &amp; ci", IMAGE)).toBe(
      "@Lambda fix conflicts & ci",
    );
    expect(plainTriggerSummary("a &lt;b&gt; &quot;c&quot; &#39;d&#39;", IMAGE)).toBe(
      `a <b> "c" 'd'`,
    );
  });

  it("decodes in one pass, so an escaped entity reads as typed", () => {
    expect(plainTriggerSummary("write &amp;lt; for <", IMAGE)).toBe("write &lt; for <");
  });

  it("replaces a whole image with the image label", () => {
    expect(
      plainTriggerSummary("![CleanShot.png](https://example.com/a.png) 这里的间距不对", IMAGE),
    ).toBe("[Image] 这里的间距不对");
  });

  it("replaces an image the snapshot cut mid-way", () => {
    // The server cuts at ~200 runes and appends "…", usually inside the URL.
    expect(
      plainTriggerSummary("看看这个 ![CleanShot 2026-09-26 at 23.25.40@2x.png](https://multica-app.example/api/attach…", IMAGE),
    ).toBe("看看这个 [Image]");
    expect(plainTriggerSummary("![CleanShot 2026-09-26 at 23.2…", IMAGE)).toBe("[Image]");
  });

  it("keeps a link's text and a mention's name", () => {
    expect(
      plainTriggerSummary("@[docs](https://example.com) and [@Lambda](mention://agent/abc) please", IMAGE),
    ).toBe("@docs and @Lambda please");
    expect(plainTriggerSummary("see [the spec](https://example.com/very/lo…", IMAGE)).toBe(
      "see the spec…",
    );
  });

  it("collapses whitespace into one line", () => {
    expect(plainTriggerSummary("  first   line\n\nsecond ", IMAGE)).toBe("first line second");
  });
});

describe("runNumbersByTask", () => {
  const task = (id: string, agent_id: string, created_at: string) =>
    ({ id, agent_id, created_at }) as AgentTask;

  it("numbers each agent's runs on the issue by when they were created", () => {
    const numbers = runNumbersByTask([
      task("pieter-3", "pieter", "2026-10-13T12:00:00Z"),
      task("anouk-1", "anouk", "2026-10-13T10:30:00Z"),
      task("pieter-1", "pieter", "2026-10-13T10:00:00Z"),
      task("pieter-2", "pieter", "2026-10-13T11:00:00Z"),
    ]);
    expect(numbers.get("pieter-1")).toBe(1);
    expect(numbers.get("pieter-2")).toBe(2);
    expect(numbers.get("pieter-3")).toBe(3);
    expect(numbers.get("anouk-1")).toBe(1);
  });
});
