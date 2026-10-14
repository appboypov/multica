package issuetemplate

import (
	"regexp"
	"strings"
)

// scenarioSections are the per-scenario sections the scenarios format adds
// under each scenario line.
var scenarioSections = []Section{
	{Heading: "#### 👤 User Story", Question: "Who benefits and what do they want to achieve?"},
	{Heading: "#### ✨ Scenario", Question: "What do the sequences of { 👤 User, 🧠 System, 🎨 Screen } [ verb ] steps look like when this exists?"},
	{Heading: "#### 📍 Currently", Question: "What is the current state?"},
	{Heading: "#### 🎯 Should", Question: "What should the state be?"},
	{Heading: "#### ⚠️ Constraints", Question: "What limitations or constraints exist?"},
	{Heading: "#### ✅ Acceptance Criteria", Question: "How do we measure successful achievement of this scenario?"},
}

// listMark matches a list mark typed at the start of a line: "- ", "* ",
// "+ ", an optional "[ ] " or "[x] " after it, or "1. " and "1) ".
var listMark = regexp.MustCompile(`^\s*(?:[-*+]\s+(?:\[[ xX]\]\s+)?|\d+[.)]\s+)`)

// Markdown is the template's Markdown with the entered fields converted into
// the sections they fill. A section without a field, or with an empty one,
// keeps its heading and question. Sections are joined by a blank line.
func Markdown(t Template, fields map[string]string) string {
	parts := make([]string, 0, len(t.Sections))
	for _, s := range t.Sections {
		parts = append(parts, sectionMarkdown(s, fields))
	}
	return strings.Join(parts, "\n\n")
}

func sectionMarkdown(s Section, fields map[string]string) string {
	if s.Field == nil {
		return section(s, "")
	}
	text := strings.TrimSpace(fields[s.Field.Key])
	if text == "" {
		return section(s, "")
	}
	switch s.Field.Format {
	case FormatBullets:
		return section(s, prefixed(lines(text), "- "))
	case FormatChecks:
		return section(s, prefixed(lines(text), "- [ ] "))
	case FormatScenarios:
		parts := []string{section(s, "")}
		for _, line := range lines(text) {
			scenario := []string{"### 👤 " + line}
			for _, sub := range scenarioSections {
				scenario = append(scenario, section(sub, ""))
			}
			parts = append(parts, strings.Join(scenario, "\n\n"))
		}
		return strings.Join(parts, "\n\n")
	default:
		return section(s, text)
	}
}

// section is a heading, its question as a quote, and the body after a blank
// line. An empty question or body is left out.
func section(s Section, body string) string {
	parts := []string{s.Heading}
	if s.Question != "" {
		parts = append(parts, "> "+s.Question)
	}
	if body != "" {
		parts = append(parts, "\n"+body)
	}
	return strings.Join(parts, "\n")
}

// lines splits text into its non-empty lines with typed list marks stripped.
func lines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(listMark.ReplaceAllString(line, ""))
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func prefixed(items []string, prefix string) string {
	for i, item := range items {
		items[i] = prefix + item
	}
	return strings.Join(items, "\n")
}
