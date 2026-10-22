package workpath

import (
	"errors"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestProjectPaths(t *testing.T) {
	p := Project{Root: "~/hq", Organisation: "sm", Slug: "supermarty"}
	cases := []struct {
		name string
		got  func() (string, error)
		want string
	}{
		{"checkout", func() (string, error) { return p.Checkout("appboypov/supermarty") }, "~/hq/sm/supermarty/repos/supermarty"},
		{"lane", func() (string, error) { return p.Worktree("appboypov/supermarty", "rooster") }, "~/hq/sm/supermarty/worktrees/supermarty/rooster"},
		{"issue", func() (string, error) { return p.Worktree("appboypov/supermarty", "sm-12") }, "~/hq/sm/supermarty/worktrees/supermarty/sm-12"},
		{"pull requests", func() (string, error) { return p.PullRequests("appboypov/supermarty") }, "~/hq/sm/supermarty/worktrees/supermarty/.pull-requests"},
	}
	for _, tc := range cases {
		got, err := tc.got()
		if err != nil || got != tc.want {
			t.Errorf("%s = %q, %v, want %q", tc.name, got, err, tc.want)
		}
	}
}

func TestProjectDefaultsRootAndKeepsAbsoluteRoot(t *testing.T) {
	got, err := Project{Organisation: "das", Slug: "multica"}.Checkout("appboypov/multica")
	if err != nil || got != "~/hq/das/multica/repos/multica" {
		t.Fatalf("default root checkout = %q, %v", got, err)
	}
	got, err = Project{Root: "/work", Organisation: "das", Slug: "multica"}.Worktree("appboypov/multica", "work-1")
	if err != nil || got != "/work/das/multica/worktrees/multica/work-1" {
		t.Fatalf("absolute root worktree = %q, %v", got, err)
	}
}

func TestProjectRequiresOrganisationAndSlug(t *testing.T) {
	if _, err := (Project{Slug: "x"}).Worktree("o/r", "f"); !errors.Is(err, ErrNoOrganisation) {
		t.Fatalf("no organisation err = %v", err)
	}
	if _, err := (Project{Organisation: "o"}).Worktree("o/r", "f"); !errors.Is(err, ErrNoProjectSlug) {
		t.Fatalf("no slug err = %v", err)
	}
	for _, p := range []Project{{Organisation: "..", Slug: "x"}, {Organisation: "o", Slug: "Bad"}, {Root: "rel", Organisation: "o", Slug: "x"}} {
		if _, err := p.Dir(); err == nil {
			t.Errorf("%+v: want an error", p)
		}
	}
	if _, err := (Project{Organisation: "o", Slug: "x"}).Worktree("o/r", ".."); err == nil {
		t.Fatal("folder .. accepted")
	}
}

func TestValidSlug(t *testing.T) {
	for slug, want := range map[string]bool{"amd": true, "ivga-amd": true, "2go": true, "-x": false, "Amd": false, "a_b": false, "": false} {
		if ValidSlug(slug) != want {
			t.Errorf("ValidSlug(%q) = %v", slug, !want)
		}
	}
}

func TestRootFromSettings(t *testing.T) {
	cases := map[string]string{
		``:                         DefaultRoot,
		`{}`:                       DefaultRoot,
		`{"work_root":"  "}`:       DefaultRoot,
		`{"work_root":"rel/dir"}`:  DefaultRoot,
		`{"work_root":"~/a/../b"}`: DefaultRoot,
		`{"work_root":"~/work/"}`:  "~/work",
		`{"work_root":"/srv/hq"}`:  "/srv/hq",
	}
	for settings, want := range cases {
		if got := RootFromSettings([]byte(settings)); got != want {
			t.Errorf("RootFromSettings(%s) = %q, want %q", settings, got, want)
		}
	}
}

func TestRelocationApply(t *testing.T) {
	home := "/Users/b"
	r := Relocation{
		OldCheckout: "~/Repos/Clients/amd",
		NewCheckout: "~/hq/ivga/amd/repos/ivga-amd",
		Prefixes: []protocol.WorktreePrefix{
			{From: "~/Worktrees/ivga/ivga-amd", To: "~/hq/ivga/amd/worktrees/ivga-amd"},
			{From: "/Users/b/Worktrees/ivga/ivga-amd/special", To: "/elsewhere"},
		},
	}
	cases := []struct {
		stored, want string
		moved        bool
	}{
		// A lane on the old checkout follows the new checkout.
		{"~/Repos/Clients/amd", "~/hq/ivga/amd/repos/ivga-amd", true},
		{"/Users/b/Repos/Clients/amd/", "/Users/b/hq/ivga/amd/repos/ivga-amd", true},
		// Prefix replacement keeps the stored style after ~ expansion.
		{"~/Worktrees/ivga/ivga-amd/work-1", "~/hq/ivga/amd/worktrees/ivga-amd/work-1", true},
		{"/Users/b/Worktrees/ivga/ivga-amd/.pull-requests/feat/x", "/Users/b/hq/ivga/amd/worktrees/ivga-amd/.pull-requests/feat/x", true},
		// The longest prefix wins.
		{"~/Worktrees/ivga/ivga-amd/special/a", "/elsewhere/a", true},
		// A boundary that only shares a string prefix does not match.
		{"~/Worktrees/ivga/ivga-amd-old/work-1", "~/Worktrees/ivga/ivga-amd-old/work-1", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, moved := r.Apply(tc.stored, home)
		if got != tc.want || moved != tc.moved {
			t.Errorf("Apply(%q) = %q, %v, want %q, %v", tc.stored, got, moved, tc.want, tc.moved)
		}
	}
}
