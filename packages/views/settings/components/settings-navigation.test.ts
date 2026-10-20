// @vitest-environment node
import { describe, expect, it } from "vitest";
import { resolveSettingsLocation, settingsHref } from "./settings-navigation";

const resolve = (search: string) =>
  resolveSettingsLocation(new URLSearchParams(search));

describe("settings location", () => {
  it.each([
    ["tab=issue", { tab: "preferences", section: "issue" }],
    ["tab=chat", { tab: "preferences", section: "chat" }],
    ["tab=labs", { tab: "workspace", section: null }],
    ["tab=integrations", { tab: "apps", section: null }],
    [
      "tab=integrations&integration=vcs",
      { tab: "code", section: "code-hosting" },
    ],
    [
      "tab=integrations&integration=github",
      { tab: "code", section: null },
    ],
    [
      "tab=integrations&integration=composio",
      { tab: "apps", section: null },
    ],
  ])("resolves the retired %s entry", (search, expected) => {
    expect(resolve(search)).toEqual(expected);
  });

  it("keeps server redirects from the GitHub App install working", () => {
    // github.go returns to one of these two tabs after the App is installed.
    expect(resolve("tab=github")).toEqual({
      tab: "code",
      section: null,
    });
    expect(resolve("tab=repositories&github_connected=1")).toEqual({
      tab: "code",
      section: "repositories",
    });
  });

  it.each(["connected=notion", "error=composio_connect_failed"])(
    "sends the Composio OAuth callback (%s) to Connected apps",
    (callback) => {
      expect(resolve(`tab=integrations&${callback}`).tab).toBe("apps");
    },
  );

  it("passes current pages and anchors through untouched", () => {
    expect(resolve("tab=code&section=pr-sidebar")).toEqual({
      tab: "code",
      section: "pr-sidebar",
    });
    expect(resolve("").tab).toBe("profile");
  });

  it("preserves unrelated query state while replacing page-specific state", () => {
    expect(
      settingsHref(
        "/acme/settings",
        new URLSearchParams("tab=integrations&integration=composio&section=chat&keep=1"),
        "code",
        { section: "pr-sidebar" },
      ),
    ).toBe("/acme/settings?tab=code&keep=1&section=pr-sidebar");
  });
});
