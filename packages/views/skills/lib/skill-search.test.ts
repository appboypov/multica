import { describe, expect, it } from "vitest";
import { filterBySkillSearch } from "./skill-search";

const skills = [
  { name: "Review", description: "Reviews code", path: "~/skills/review" },
  { name: "Plan", description: null, path: "~/team/plan" },
];

const fields = (s: (typeof skills)[number]) => [s.name, s.description, s.path];

describe("filterBySkillSearch", () => {
  it("keeps every item for an empty or blank query", () => {
    expect(filterBySkillSearch(skills, "  ", fields)).toEqual(skills);
  });

  it("matches any field without regard to case or surrounding spaces", () => {
    expect(filterBySkillSearch(skills, " CODE ", fields)).toEqual([skills[0]]);
    expect(filterBySkillSearch(skills, "team", fields)).toEqual([skills[1]]);
  });

  it("skips missing fields and returns nothing without a match", () => {
    expect(filterBySkillSearch(skills, "missing", fields)).toEqual([]);
  });
});
