// @vitest-environment jsdom

import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import enAgents from "../../locales/en/agents.json";
import { CharCounter } from "./char-counter";

function renderCounter(props: { length: number; max: number; soft?: boolean }) {
  const { container } = render(
    <I18nProvider locale="en" resources={{ en: { agents: enAgents } }}>
      <CharCounter {...props} />
    </I18nProvider>,
  );
  return container.firstElementChild as HTMLElement;
}

describe("CharCounter", () => {
  afterEach(cleanup);

  it("warns past a soft threshold with the over-threshold text", () => {
    const counter = renderCounter({ length: 300, max: 255, soft: true });

    expect(counter).toHaveTextContent("300 / 255 · 45 over the recommended 255");
    expect(counter).toHaveClass("text-warning");
    expect(counter).not.toHaveClass("text-destructive");
  });

  it("errors past a hard limit with the over-limit text", () => {
    const counter = renderCounter({ length: 300, max: 255 });

    expect(counter).toHaveTextContent("300 / 255 · 45 over limit");
    expect(counter).toHaveClass("text-destructive");
  });

  it("stays neutral near and at a soft threshold", () => {
    expect(renderCounter({ length: 240, max: 255, soft: true })).toHaveClass(
      "text-muted-foreground",
    );
    cleanup();
    const atThreshold = renderCounter({ length: 255, max: 255, soft: true });
    expect(atThreshold).toHaveClass("text-muted-foreground");
    expect(atThreshold).toHaveTextContent("255 / 255");
    expect(atThreshold).not.toHaveTextContent("recommended");
  });

  it("keeps the near tone of a hard limit", () => {
    expect(renderCounter({ length: 240, max: 255 })).toHaveClass("text-warning");
  });
});
