import { describe, expect, it } from "vitest";
import { parseCurrentContextRoute } from "./use-chat-context-items";

describe("parseCurrentContextRoute", () => {
  it("takes the selected to-do on Inbox as the current context", () => {
    expect(parseCurrentContextRoute("/acme/inbox", new URLSearchParams("todo=todo-1"))).toEqual({ type: "todo", id: "todo-1" });
  });

  it("takes the selected issue on Updates as the current issue", () => {
    expect(parseCurrentContextRoute("/acme/updates", new URLSearchParams("issue=issue-1"))).toEqual({ type: "issue", id: "issue-1" });
  });

  it("has no current context on Inbox without a selected to-do", () => {
    expect(parseCurrentContextRoute("/acme/inbox", new URLSearchParams("issue=issue-1"))).toBeNull();
  });
});
