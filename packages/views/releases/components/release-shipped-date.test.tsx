import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithI18n } from "../../test/i18n";
import { ReleaseShippedDate } from "./release-shipped-date";

describe("release ship date", () => {
  it("shows the ship date with its machine-readable timestamp", () => {
    const shippedAt = "2026-10-15T12:00:00Z";
    renderWithI18n(<ReleaseShippedDate shippedAt={shippedAt} />);
    const date = screen.getByText(new Date(shippedAt).toLocaleDateString("en", { dateStyle: "medium" }));
    expect(date.tagName).toBe("TIME");
    expect(date).toHaveAttribute("dateTime", shippedAt);
  });

  it("shows no ship date for an unshipped release", () => {
    const { container } = renderWithI18n(<ReleaseShippedDate shippedAt={null} />);
    expect(container.querySelector("time")).toBeNull();
  });
});
