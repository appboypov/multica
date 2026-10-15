package question

import (
	"fmt"
	"strings"
)

// Link types of question_link.link_type (design.md decision 1, migrations 710
// and 711). A sub-issue is an issue link to an issue with a parent; a run is a
// task link; a chat is a chat session, where a question asked in a chat turn
// is asked.
const (
	LinkTypeIssue     = "issue"
	LinkTypeTask      = "task"
	LinkTypeComment   = "comment"
	LinkTypeProject   = "project"
	LinkTypeAgent     = "agent"
	LinkTypeSquad     = "squad"
	LinkTypeAutopilot = "autopilot"
	LinkTypeSkill     = "skill"
	LinkTypeChat      = "chat"
)

// MaxLinksPerAsk bounds the links one request may give: an ask's links, and the
// links one add appends to a question.
const MaxLinksPerAsk = 10

// LinkRef is a link before its object is resolved: Type is a link type and Ref
// the object's id, or what the CLI's resolver of that type takes (a key, a
// name or a unique id prefix).
type LinkRef struct {
	Type string
	Ref  string
}

// One registry names every link type and the prefix the CLI writes it with.
// Only a run differs: the CLI writes run:, the wire and the database task.
var linkTypes = []struct {
	linkType string
	prefix   string
}{
	{LinkTypeIssue, "issue"},
	{LinkTypeTask, "run"},
	{LinkTypeComment, "comment"},
	{LinkTypeProject, "project"},
	{LinkTypeAgent, "agent"},
	{LinkTypeSquad, "squad"},
	{LinkTypeAutopilot, "autopilot"},
	{LinkTypeSkill, "skill"},
	{LinkTypeChat, "chat"},
}

// ValidLinkType reports whether linkType is one of the nine link types.
func ValidLinkType(linkType string) bool {
	for _, def := range linkTypes {
		if def.linkType == linkType {
			return true
		}
	}
	return false
}

// ParseLinkRef reads the CLI's form of a link, "<prefix>:<ref>" or a bare issue
// key or id: "project:01J…" is a project link, "run:<id>" a task link and
// "WORK-415" an issue link. An unknown prefix or an empty ref is refused; the
// ref itself is resolved by the caller.
func ParseLinkRef(value string) (LinkRef, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return LinkRef{}, fmt.Errorf("empty link: give an issue key or <type>:<ref> (%s)", linkPrefixList())
	}
	prefix, ref, found := strings.Cut(value, ":")
	if !found {
		return LinkRef{Type: LinkTypeIssue, Ref: value}, nil
	}
	for _, def := range linkTypes {
		if def.prefix != prefix {
			continue
		}
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return LinkRef{}, fmt.Errorf("link %q has no reference after %s:", value, prefix)
		}
		return LinkRef{Type: def.linkType, Ref: ref}, nil
	}
	return LinkRef{}, fmt.Errorf("unknown link type %q in %q: use %s", prefix, value, linkPrefixList())
}

func linkPrefixList() string {
	prefixes := make([]string, len(linkTypes))
	for i, def := range linkTypes {
		prefixes[i] = def.prefix + ":"
	}
	return strings.Join(prefixes, ", ")
}
