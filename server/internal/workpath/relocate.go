package workpath

import (
	"path"
	"strings"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Relocation is a repository's move to a new checkout (ADR-0037). Every
// recorded worktree whose path equals the old checkout follows the new
// checkout; every other one at or under a prefix's From moves to the same place
// under its To, the longest matching From winning. The server and the daemon
// both apply it here, so a check's probe and the write that follows it agree.
type Relocation struct {
	// OldCheckout and NewCheckout are the repository's checkout before and
	// after the move, in any style ("~/..." or absolute).
	OldCheckout string
	NewCheckout string
	Prefixes    []protocol.WorktreePrefix
}

// Expand returns p absolute and clean, with a leading "~" read as home.
func Expand(p, home string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		p = home + strings.TrimPrefix(p, "~")
	}
	return path.Clean(p)
}

// under reports whether p is base or lies below it, both absolute and clean.
func under(p, base string) bool {
	return p == base || base == "/" || strings.HasPrefix(p, base+"/")
}

// Apply returns where the worktree recorded at stored goes, in the stored
// path's style: a "~/" path stays "~/" while it lands under home. moved is
// false when the relocation leaves the path where it is.
func (r Relocation) Apply(stored, home string) (relocated string, moved bool) {
	if stored == "" {
		return stored, false
	}
	current := Expand(stored, home)
	var target string
	if r.OldCheckout != "" && current == Expand(r.OldCheckout, home) && r.NewCheckout != "" {
		target = Expand(r.NewCheckout, home)
	} else {
		best := -1
		for _, prefix := range r.Prefixes {
			from := Expand(prefix.From, home)
			if !under(current, from) || len(from) <= best {
				continue
			}
			best = len(from)
			target = path.Join(Expand(prefix.To, home), strings.TrimPrefix(current, from))
		}
		if best < 0 {
			return stored, false
		}
	}
	if target == current {
		return stored, false
	}
	if strings.HasPrefix(stored, "~") && home != "" && under(target, path.Clean(home)) {
		if target == path.Clean(home) {
			return "~", true
		}
		return "~/" + strings.TrimPrefix(target, path.Clean(home)+"/"), true
	}
	return target, true
}

// ValidPrefix reports whether both sides of a prefix are valid roots: an
// absolute path or a path written with "~/".
func ValidPrefix(prefix protocol.WorktreePrefix) bool {
	return ValidRoot(prefix.From) && ValidRoot(prefix.To)
}
