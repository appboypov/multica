// Package releaseline holds the rules of a repository's release line
// (ADR-0010): renumbering computed versions, checking that the line stays
// strictly increasing, and planning, editing, moving and removing a release.
// It is pure: no database and no HTTP. Versions are written without the `v`.
package releaseline

import (
	"strconv"
	"strings"
)

// Bumps of a computed release. An empty Bump means the version was set by
// hand.
const (
	BumpPatch = "patch"
	BumpMinor = "minor"
	BumpMajor = "major"
)

// Directions of a move.
const (
	Earlier = "earlier"
	Later   = "later"
)

// Problem codes, as the API answers them.
const (
	CodeFirstRelease      = "first_release"
	CodeLastRelease       = "last_release"
	CodeLineNotIncreasing = "line_not_increasing"
	CodeNotOnLine         = "release_not_on_line"
	CodeShipped           = "release_shipped"
	CodeBeforeShipped     = "release_before_shipped"
	CodeNotNext           = "release_not_next"
)

// Release is one release of a line, in line order.
type Release struct {
	ID      string
	Version string
	// Bump is patch, minor or major for a computed version, or "" when the
	// version was set by hand.
	Bump    string
	Title   string
	Shipped bool
}

// Line is a repository's release line. LastTag is the repository's last tag
// without the `v`, or "" when it has none.
type Line struct {
	LastTag  string
	Releases []Release
}

// Problem is why a change would break the line. For line_not_increasing,
// Release is the first release whose version is not higher than the one
// before it, and Before is that release, or nil when the last tag stands
// before it.
type Problem struct {
	Code    string
	Release Release
	Before  *Release
	LastTag string
}

// Change is a release whose version changed.
type Change struct {
	ReleaseID string `json:"release_id"`
	Title     string `json:"title"`
	From      string `json:"from"`
	To        string `json:"to"`
}

// Version is a semantic version of three integers.
type Version struct {
	Major, Minor, Patch int
}

// ParseVersion accepts `v1.4.0` and `1.4.0`, with surrounding spaces.
func ParseVersion(s string) (Version, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	var n [3]int
	for i, p := range parts {
		if p == "" {
			return Version{}, false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return Version{}, false
			}
		}
		v, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, false
		}
		n[i] = v
	}
	return Version{Major: n[0], Minor: n[1], Patch: n[2]}, true
}

// String writes the version without the `v`.
func (v Version) String() string {
	return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
}

// Compare is negative, zero or positive as v is lower than, equal to or
// higher than o.
func (v Version) Compare(o Version) int {
	switch {
	case v.Major != o.Major:
		return sign(v.Major - o.Major)
	case v.Minor != o.Minor:
		return sign(v.Minor - o.Minor)
	default:
		return sign(v.Patch - o.Patch)
	}
}

// Next is the version bump gives after v.
func (v Version) Next(bump string) Version {
	switch bump {
	case BumpMajor:
		return Version{Major: v.Major + 1}
	case BumpMinor:
		return Version{Major: v.Major, Minor: v.Minor + 1}
	default:
		return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
	}
}

// ValidBump reports whether b is patch, minor or major.
func ValidBump(b string) bool {
	return b == BumpPatch || b == BumpMinor || b == BumpMajor
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// parseOrZero reads a stored version; a stored version is always valid, so
// the zero version only stands in for an empty last tag.
func parseOrZero(s string) Version {
	v, _ := ParseVersion(s)
	return v
}

// Renumber gives each unshipped release with a bump the version its bump
// gives after the version before it. The version before the first release is
// the last tag, or 0.0.0 when there is none. A release set by hand, and every
// shipped release, keeps its version.
func Renumber(line Line) Line {
	out := Line{LastTag: line.LastTag, Releases: make([]Release, len(line.Releases))}
	prev := parseOrZero(line.LastTag)
	for i, r := range line.Releases {
		if !r.Shipped && r.Bump != "" {
			r.Version = prev.Next(r.Bump).String()
		}
		out.Releases[i] = r
		prev = parseOrZero(r.Version)
	}
	return out
}

// Check finds the first release whose version is not higher than the one
// before it. Before the first release stands the last tag; when the last tag
// is "", the first release has no lower bound.
func Check(line Line) *Problem {
	for i, r := range line.Releases {
		v := parseOrZero(r.Version)
		if i == 0 {
			if line.LastTag == "" {
				continue
			}
			if v.Compare(parseOrZero(line.LastTag)) <= 0 {
				return &Problem{Code: CodeLineNotIncreasing, Release: r, LastTag: line.LastTag}
			}
			continue
		}
		before := line.Releases[i-1]
		if v.Compare(parseOrZero(before.Version)) <= 0 {
			return &Problem{Code: CodeLineNotIncreasing, Release: r, Before: &before, LastTag: line.LastTag}
		}
	}
	return nil
}

// Plan adds r at the end of the line, then renumbers and checks.
func Plan(line Line, r Release) (Line, *Problem) {
	releases := make([]Release, 0, len(line.Releases)+1)
	releases = append(releases, line.Releases...)
	releases = append(releases, r)
	return settle(Line{LastTag: line.LastTag, Releases: releases})
}

// Edit replaces the version, bump and title of the release id in place, then
// renumbers and checks. A new bump drops the version set by hand.
func Edit(line Line, id string, r Release) (Line, *Problem) {
	i := indexOf(line, id)
	if i < 0 {
		return line, &Problem{Code: CodeNotOnLine}
	}
	current := line.Releases[i]
	if current.Shipped && (r.Bump != current.Bump || (r.Bump == "" && r.Version != current.Version)) {
		return line, &Problem{Code: CodeShipped, Release: current}
	}
	releases := clone(line.Releases)
	if !current.Shipped {
		releases[i].Version = r.Version
	}
	releases[i].Bump = r.Bump
	releases[i].Title = r.Title
	return settle(Line{LastTag: line.LastTag, Releases: releases})
}

// Move swaps the release id with its neighbour in direction, then renumbers
// and checks. It refuses first_release and last_release first.
func Move(line Line, id, direction string) (Line, *Problem) {
	i := indexOf(line, id)
	if i < 0 {
		return line, &Problem{Code: CodeNotOnLine}
	}
	if line.Releases[i].Shipped {
		return line, &Problem{Code: CodeShipped, Release: line.Releases[i]}
	}
	j := i + 1
	if direction == Earlier {
		if i == 0 {
			return line, &Problem{Code: CodeFirstRelease, Release: line.Releases[i]}
		}
		j = i - 1
	} else if i == len(line.Releases)-1 {
		return line, &Problem{Code: CodeLastRelease, Release: line.Releases[i]}
	}
	if line.Releases[j].Shipped {
		return line, &Problem{Code: CodeBeforeShipped, Release: line.Releases[i]}
	}
	releases := clone(line.Releases)
	releases[i], releases[j] = releases[j], releases[i]
	return settle(Line{LastTag: line.LastTag, Releases: releases})
}

// Remove takes the release id off the line, then renumbers. Removing cannot
// make a version too low, but the check runs anyway.
func Remove(line Line, id string) (Line, *Problem) {
	i := indexOf(line, id)
	if i < 0 {
		return line, &Problem{Code: CodeNotOnLine}
	}
	if line.Releases[i].Shipped {
		return line, &Problem{Code: CodeShipped, Release: line.Releases[i]}
	}
	releases := make([]Release, 0, len(line.Releases)-1)
	releases = append(releases, line.Releases[:i]...)
	releases = append(releases, line.Releases[i+1:]...)
	return settle(Line{LastTag: line.LastTag, Releases: releases})
}

// Ship fixes the version of the first unshipped release without renumbering.
func Ship(line Line, id string) (Line, *Problem) {
	i := indexOf(line, id)
	if i < 0 {
		return line, &Problem{Code: CodeNotOnLine}
	}
	if line.Releases[i].Shipped {
		return line, &Problem{Code: CodeShipped, Release: line.Releases[i]}
	}
	for _, r := range line.Releases[:i] {
		if !r.Shipped {
			return line, &Problem{Code: CodeNotNext, Release: line.Releases[i]}
		}
	}
	releases := clone(line.Releases)
	releases[i].Shipped = true
	return Line{LastTag: line.LastTag, Releases: releases}, nil
}

// Changes lists, in the order of after, every release of both lines whose
// version changed, with its title after the change.
func Changes(before, after Line) []Change {
	old := make(map[string]string, len(before.Releases))
	for _, r := range before.Releases {
		old[r.ID] = r.Version
	}
	changes := []Change{}
	for _, r := range after.Releases {
		from, ok := old[r.ID]
		if !ok || from == r.Version {
			continue
		}
		changes = append(changes, Change{ReleaseID: r.ID, Title: r.Title, From: from, To: r.Version})
	}
	return changes
}

func settle(line Line) (Line, *Problem) {
	line = Renumber(line)
	if p := Check(line); p != nil {
		return line, p
	}
	return line, nil
}

func indexOf(line Line, id string) int {
	for i, r := range line.Releases {
		if r.ID == id {
			return i
		}
	}
	return -1
}

func clone(releases []Release) []Release {
	out := make([]Release, len(releases))
	copy(out, releases)
	return out
}
