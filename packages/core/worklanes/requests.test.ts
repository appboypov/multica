import { describe, expect, it } from "vitest";
import { ApiError } from "../api";
import { worklaneRefusal } from "./requests";

const refused = (status: number, body: unknown) => new ApiError("refused", status, "Refused", body);

describe("worklaneRefusal", () => {
  it("reads a coded refusal with its detail", () => {
    expect(worklaneRefusal(refused(409, { code: "name_taken", error: "taken", detail: "Fan app" }))).toMatchObject({
      code: "name_taken",
      detail: "Fan app",
    });
  });

  it("is null for an error body without a code, so the dialog's own failure copy shows", () => {
    expect(worklaneRefusal(refused(403, { error: "insufficient permissions" }))).toBeNull();
    expect(worklaneRefusal(refused(500, "internal"))).toBeNull();
    expect(worklaneRefusal(new Error("network"))).toBeNull();
  });

  it("names the open issues of Remove's immediate refusal like the request's re-check does", () => {
    const reason = worklaneRefusal(
      refused(409, {
        code: "worklane_has_open_issues",
        error: "worklane still has open issues",
        issues: [
          { id: "1", identifier: "REBL-24", title: "Login" },
          { id: "2", identifier: "REBL-27", title: "Push" },
        ],
      }),
    );
    expect(reason).toMatchObject({
      code: "worklane_has_open_issues",
      detail: "worklane still has the open issues REBL-24, REBL-27",
    });
  });
});
