import { describe, expect, it } from "vitest";
import { NextRequest } from "next/server";
import { proxy } from "../proxy";
import manifest, { PWA_START_URL } from "./manifest";

function launch(cookies: Record<string, string>) {
  const cookieHeader = Object.entries(cookies)
    .map(([key, value]) => `${key}=${value}`)
    .join("; ");

  return proxy(
    new NextRequest(`https://app.multica.test${PWA_START_URL}`, {
      headers: cookieHeader ? { cookie: cookieHeader } : undefined,
    }),
  ).headers.get("location");
}

describe("web app manifest", () => {
  it("launches into the last workspace for a signed-in session", () => {
    expect(
      launch({ multica_logged_in: "1", last_workspace_slug: "acme" }),
    ).toContain("/acme/inbox");
  });

  it("launches into login when there is no session", () => {
    expect(launch({})).toContain("/login");
  });

  it("launches into login for a session with no known workspace", () => {
    // Reachable whenever `multica_logged_in` outlives `last_workspace_slug`:
    // a member who signed up but has not opened a workspace yet, or cleared
    // cookies. /login resolves it against the workspace list.
    expect(launch({ multica_logged_in: "1" })).toContain("/login");
  });

  it("points every shortcut at a path that resolves the same way", () => {
    const shortcuts = manifest().shortcuts ?? [];

    expect(shortcuts.length).toBeGreaterThan(0);
    for (const shortcut of shortcuts) {
      const resolve = (cookie: string) =>
        proxy(
          new NextRequest(`https://app.multica.test${shortcut.url}`, {
            headers: { cookie },
          }),
        ).headers.get("location");

      expect(
        resolve("multica_logged_in=1; last_workspace_slug=acme"),
      ).toContain(`/acme${shortcut.url}`);
      // Same three states as start_url — a shortcut is a launcher entry too.
      expect(resolve("multica_logged_in=1")).toContain("/login");
      expect(resolve("")).toContain("/login");
    }
  });
});
