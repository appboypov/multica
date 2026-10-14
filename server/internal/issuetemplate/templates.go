// Package issuetemplate holds the eight issue templates and turns a template's
// filled fields into the one Markdown description an issue keeps (ADR-0012).
// It has no database or HTTP imports.
package issuetemplate

// Format names how a field's text becomes Markdown under its section.
type Format string

const (
	FormatText      Format = "text"
	FormatBullets   Format = "bullets"
	FormatChecks    Format = "checks"
	FormatScenarios Format = "scenarios"
)

// Field is the New issue field that fills one section.
type Field struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder"`
	Rows        int    `json:"rows"`
	Format      Format `json:"format"`
}

// Section is one heading of a template, with its question callout and the
// field that fills it, if any.
type Section struct {
	Heading  string `json:"heading"`
	Question string `json:"question"`
	Field    *Field `json:"field"`
}

// Template is the Markdown skeleton of one issue type.
type Template struct {
	Type     string    `json:"type"`
	Sections []Section `json:"sections"`
}

// All returns the eight templates in their fixed order.
func All() []Template {
	return templates
}

// ByType returns the template of an issue type.
func ByType(issueType string) (Template, bool) {
	for _, t := range templates {
		if t.Type == issueType {
			return t, true
		}
	}
	return Template{}, false
}

// Types returns the eight issue types in template order.
func Types() []string {
	types := make([]string, len(templates))
	for i, t := range templates {
		types[i] = t.Type
	}
	return types
}

// IsType reports whether issueType is one of the eight.
func IsType(issueType string) bool {
	_, ok := ByType(issueType)
	return ok
}

// The eight issue templates (ADR-0012), ported from TEMPLATES in
// prototypes/web/src/components/generated/model.ts without the prototype's
// name, hint and example, which are UI copy and a prototype aid.
var templates = []Template{
	{
		Type: "epic",
		Sections: []Section{
			{Heading: "## 🔗 Dependencies", Question: "Which issues must be completed first?"},
			{Heading: "## 🔀 Related Issues", Question: "Which related issues provide context (non-blocking)?"},
			{Heading: "## 🎯 Thesis & outcome", Question: "What is this body of work and what does “done” mean — using one coherent scope (single epic, theme, or umbrella with child epics, without duplicating the same story under different labels)?"},
			{Heading: "### What we are doing", Field: &Field{Key: "doing", Label: "What we are doing", Required: true, Placeholder: "The body of work, in a few sentences", Rows: 3, Format: FormatText}},
			{Heading: "### What success looks like"},
			{Heading: "## 🏢 Strategic alignment", Question: "How does this connect to company goals or OKRs?"},
			{Heading: "## 💡 Hypothesis", Question: "What do we believe will happen if we pursue this?"},
			{Heading: "## 💰 Value & investment", Question: "Why does this matter, what does it cost, and how do we judge importance?"},
			{Heading: "### Opportunity cost of inaction"},
			{Heading: "### Expected ROI"},
			{Heading: "### Impact (product & engineering)"},
			{Heading: "## 👥 Stakeholders", Question: "Who is involved and accountable?"},
			{Heading: "## 🗺️ Delivery plan", Question: "What is the timeline for the whole scope (rows may be phases, child epics, milestones, or sprint goals — use what fits)?"},
			{Heading: "## 👤 User stories", Question: "Which user stories comprise this work, in delivery order?", Field: &Field{Key: "stories", Label: "User stories", Required: false, Placeholder: "One per line, in delivery order", Rows: 3, Format: FormatChecks}},
			{Heading: "## 🎯 Abilities", Question: "What can actors do when this is complete?", Field: &Field{Key: "abilities", Label: "Abilities", Required: true, Placeholder: "{Actor} is able to {outcome}. One per line", Rows: 3, Format: FormatChecks}},
			{Heading: "## 📍 Currently", Question: "What is the baseline state across this scope?"},
			{Heading: "## 🚀 End goal", Question: "What is the tangible measurable end state?"},
			{Heading: "## ⚠️ Constraints", Question: "What hard limits, policies, or boundaries apply?"},
			{Heading: "## 🚦 Risks & dependencies", Question: "What could block or derail delivery (capture each item in the table)?"},
			{Heading: "## 📊 Success metrics", Question: "How will we know it worked?"},
			{Heading: "### Leading (short-term)"},
			{Heading: "### Lagging (long-term)"},
			{Heading: "### Concrete KPIs"},
			{Heading: "## ✅ Acceptance criteria", Question: "How do we know this scope is complete?", Field: &Field{Key: "acceptance", Label: "Acceptance criteria", Required: true, Placeholder: "One per line", Rows: 3, Format: FormatChecks}},
			{Heading: "## 📎 Related work", Question: "What related epics, docs, designs, or prior art should we link?"},
			{Heading: "## 📋 Next steps", Question: "What immediate actions come next?"},
			{Heading: "## 📚 References"},
		},
	},
	{
		Type: "journey",
		Sections: []Section{
			{Heading: "## 🗺️ User Journey", Question: "What is this journey about in one sentence?", Field: &Field{Key: "journey", Label: "User journey", Required: true, Placeholder: "What this journey is about, in one sentence", Rows: 2, Format: FormatText}},
			{Heading: "## ✨ Scenarios", Question: "What are the scenarios that make up this journey?", Field: &Field{Key: "scenarios", Label: "Scenarios", Required: true, Placeholder: "{Actor} is able to {outcome}. One per line; each gets its scenario sections", Rows: 4, Format: FormatScenarios}},
		},
	},
	{
		Type: "story",
		Sections: []Section{
			{Heading: "## 🔗 Dependencies", Question: "Which issues must be completed first?"},
			{Heading: "## 🔀 Related Issues", Question: "Which related issues provide context (non-blocking)?"},
			{Heading: "## 👤 User Story", Question: "Who benefits and what do they want to achieve?", Field: &Field{Key: "story", Label: "User story", Required: true, Placeholder: "As a {actor}, I want {action}, so that {benefit}.", Rows: 2, Format: FormatBullets}},
			{Heading: "## 🚀 End Goal", Question: "What is the tangible measurable end goal?"},
			{Heading: "## 📍 Currently", Question: "What is the current state?", Field: &Field{Key: "currently", Label: "Currently", Required: false, Placeholder: "The current state", Rows: 2, Format: FormatBullets}},
			{Heading: "## 🎯 Should", Question: "What should the state be?", Field: &Field{Key: "should", Label: "Should", Required: false, Placeholder: "The state it should be in", Rows: 2, Format: FormatBullets}},
			{Heading: "## 🗺️ User Journey", Question: "What is this journey about in one sentence?"},
			{Heading: "## ✨ Scenarios", Question: "What scenarios make up this journey (each with a mermaid sequence diagram for its flow)?"},
			{Heading: "## 🎨 UI", Question: "What does the updated UI look like in ASCII?"},
			{Heading: "## ⚠️ Constraints", Question: "What limitations or constraints exist for the overall story?"},
			{Heading: "## ✅ Acceptance Criteria", Question: "How do we measure successful achievement of the end goal?", Field: &Field{Key: "acceptance", Label: "Acceptance criteria", Required: true, Placeholder: "Given … when … then …. One per line", Rows: 3, Format: FormatChecks}},
			{Heading: "## 📚 References"},
		},
	},
	{
		Type: "bug",
		Sections: []Section{
			{Heading: "## 🔗 Dependencies", Question: "Which issues must be completed first?"},
			{Heading: "## 🔀 Related Issues", Question: "Which related issues provide context (non-blocking)?"},
			{Heading: "## 💥 Event", Question: "When did the problem occur?"},
			{Heading: "## 🦋 Expected Result", Question: "What did you expect to happen?", Field: &Field{Key: "expected", Label: "Expected result", Required: true, Placeholder: "What you expected to happen", Rows: 2, Format: FormatText}},
			{Heading: "## 🐛 Actual Result", Question: "What actually happened?", Field: &Field{Key: "actual", Label: "Actual result", Required: true, Placeholder: "What actually happened", Rows: 2, Format: FormatText}},
			{Heading: "## 👣 Steps to Reproduce", Question: "Which steps can we take to reproduce the problem?", Field: &Field{Key: "steps", Label: "Steps to reproduce", Required: false, Placeholder: "One per line", Rows: 3, Format: FormatChecks}},
			{Heading: "## 🌍 Environment", Question: "Where did the bug occur (OS, browser, app version, device, authentication state, etc.)?", Field: &Field{Key: "environment", Label: "Environment", Required: false, Placeholder: "OS, browser, app version, device, signed in or not", Rows: 2, Format: FormatBullets}},
			{Heading: "## ✅ Acceptance Criteria", Question: "How do we measure the successful fix of the bug?"},
			{Heading: "## 📚 References"},
		},
	},
	{
		Type: "enhancement",
		Sections: []Section{
			{Heading: "## 🔗 Dependencies", Question: "Which issues must be completed first?"},
			{Heading: "## 🔀 Related Issues", Question: "Which related issues provide context (non-blocking)?"},
			{Heading: "## ✨ Enhancement", Question: "What existing capability are we improving, and what changes for users or operators?", Field: &Field{Key: "enhancement", Label: "Enhancement", Required: true, Placeholder: "The capability, and what changes for whom", Rows: 3, Format: FormatText}},
			{Heading: "## 💡 Motivation", Question: "Why does this improvement matter now (pain, opportunity, feedback, metrics)?"},
			{Heading: "## 📦 Scope"},
			{Heading: "### In scope"},
			{Heading: "### Out of scope"},
			{Heading: "## 📍 Current behavior", Question: "How does it work today?", Field: &Field{Key: "current", Label: "Current behavior", Required: false, Placeholder: "How it works today", Rows: 2, Format: FormatBullets}},
			{Heading: "## 🎯 Desired behavior", Question: "How should it work after this enhancement?", Field: &Field{Key: "desired", Label: "Desired behavior", Required: false, Placeholder: "How it should work", Rows: 2, Format: FormatBullets}},
			{Heading: "## ⚠️ Constraints", Question: "What compatibility, performance budgets, design system rules, release timing, or platform limits apply?"},
			{Heading: "## ✅ Acceptance criteria", Question: "How do we know the enhancement is done and acceptable?", Field: &Field{Key: "acceptance", Label: "Acceptance criteria", Required: true, Placeholder: "One per line", Rows: 3, Format: FormatChecks}},
			{Heading: "## 📊 Success signals", Question: "What metrics, feedback, or qualitative checks after release would show success (optional)?"},
			{Heading: "## 📚 References"},
		},
	},
	{
		Type: "refactor",
		Sections: []Section{
			{Heading: "## 🔗 Dependencies", Question: "Which issues must be completed first?"},
			{Heading: "## 🔀 Related Issues", Question: "Which related issues provide context (non-blocking)?"},
			{Heading: "## 🔁 Refactor", Question: "What structure are we changing and why (internal reshaping, not new capability unless listed in scope)?", Field: &Field{Key: "refactor", Label: "Refactor", Required: true, Placeholder: "What changes shape, and why now", Rows: 3, Format: FormatText}},
			{Heading: "## 🤝 Behavioral contract", Question: "What must stay the same for callers, users, or APIs, and are there any intentional behavior changes (rare — call them out)?", Field: &Field{Key: "contract", Label: "Behavioral contract", Required: true, Placeholder: "What stays the same. One per line", Rows: 3, Format: FormatBullets}},
			{Heading: "## 📦 Scope"},
			{Heading: "### In scope"},
			{Heading: "### Out of scope"},
			{Heading: "## ✅ Acceptance criteria", Question: "How do we know the refactor is complete and safe?", Field: &Field{Key: "acceptance", Label: "Acceptance criteria", Required: false, Placeholder: "One per line", Rows: 3, Format: FormatChecks}},
			{Heading: "## 📚 References"},
		},
	},
	{
		Type: "chore",
		Sections: []Section{
			{Heading: "## 🔗 Dependencies", Question: "Which issues must be completed first?"},
			{Heading: "## 🔀 Related Issues", Question: "Which related issues provide context (non-blocking)?"},
			{Heading: "## 🧹 Chore", Question: "What maintenance or housekeeping is this, and why now (not a user-facing feature or bug fix)?", Field: &Field{Key: "chore", Label: "Chore", Required: true, Placeholder: "What needs doing, and why now", Rows: 3, Format: FormatText}},
			{Heading: "## 📦 Scope"},
			{Heading: "### In scope"},
			{Heading: "### Out of scope"},
			{Heading: "## 📍 Baseline", Question: "What exists before this chore?", Field: &Field{Key: "baseline", Label: "Baseline", Required: false, Placeholder: "What exists before", Rows: 2, Format: FormatBullets}},
			{Heading: "## 🎯 Target state", Question: "What exists after this chore?", Field: &Field{Key: "target", Label: "Target state", Required: false, Placeholder: "What exists after", Rows: 2, Format: FormatBullets}},
			{Heading: "## ⚠️ Blast radius & safety", Question: "What could break, who is affected, and how to roll back or verify nothing regressed?"},
			{Heading: "## ✅ Acceptance criteria", Question: "How do we know this chore is done?", Field: &Field{Key: "acceptance", Label: "Acceptance criteria", Required: true, Placeholder: "One per line", Rows: 3, Format: FormatChecks}},
			{Heading: "## 📚 References"},
		},
	},
	{
		Type: "spike",
		Sections: []Section{
			{Heading: "## 🔬 Spike Objective", Question: "What question(s) are we trying to answer?", Field: &Field{Key: "objective", Label: "Spike objective", Required: true, Placeholder: "What we need to know before we decide", Rows: 3, Format: FormatText}},
			{Heading: "## ❓ Research Questions", Question: "What specific unknowns need to be resolved?", Field: &Field{Key: "questions", Label: "Research questions", Required: false, Placeholder: "One per line", Rows: 3, Format: FormatChecks}},
			{Heading: "## 🎯 Success Criteria", Question: "How do we know the spike is complete?"},
			{Heading: "## ⏱️ Time Box", Question: "How much time is allocated for this investigation?", Field: &Field{Key: "timebox", Label: "Time box", Required: true, Placeholder: "For example: half a day", Rows: 1, Format: FormatText}},
			{Heading: "## 📚 Resources", Question: "What documentation, repos, or contacts might help?"},
			{Heading: "## 📝 Expected Output", Question: "What artifacts will be produced?"},
			{Heading: "## 🔗 Related Tickets", Question: "Which work is blocked pending this spike?"},
		},
	},
}
