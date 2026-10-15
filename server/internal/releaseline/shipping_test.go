package releaseline

import "testing"

func TestShippingFreezesTheReleaseLine(t *testing.T) {
	line := Line{LastTag: "1.0.0", Releases: []Release{
		{ID: "first", Version: "1.1.0", Bump: BumpMinor, Title: "First"},
		{ID: "next", Version: "1.1.1", Bump: BumpPatch, Title: "Next"},
	}}
	if _, p := Ship(line, "next"); p == nil || p.Code != CodeNotNext {
		t.Fatalf("shipping out of order = %+v", p)
	}
	shipped, p := Ship(line, "first")
	if p != nil || !shipped.Releases[0].Shipped || line.Releases[0].Shipped || shipped.Releases[1].Version != "1.1.1" {
		t.Fatalf("ship = %+v, %+v; input = %+v", shipped, p, line)
	}
	for name, operation := range map[string]func() (Line, *Problem){
		"ship twice":           func() (Line, *Problem) { return Ship(shipped, "first") },
		"move shipped":         func() (Line, *Problem) { return Move(shipped, "first", Later) },
		"remove shipped":       func() (Line, *Problem) { return Remove(shipped, "first") },
		"edit shipped version": func() (Line, *Problem) { return Edit(shipped, "first", Release{Version: "2.0.0"}) },
		"edit shipped bump":    func() (Line, *Problem) { return Edit(shipped, "first", Release{Bump: BumpMajor}) },
	} {
		t.Run(name, func(t *testing.T) {
			if _, p := operation(); p == nil || p.Code != CodeShipped {
				t.Fatalf("problem = %+v", p)
			}
		})
	}
	if _, p := Move(shipped, "next", Earlier); p == nil || p.Code != CodeBeforeShipped {
		t.Fatalf("crossing shipped boundary = %+v", p)
	}
	edited, p := Edit(shipped, "first", Release{Bump: BumpMinor, Title: "Published"})
	if p != nil || edited.Releases[0].Version != "1.1.0" || edited.Releases[0].Title != "Published" {
		t.Fatalf("title edit = %+v, %+v", edited, p)
	}
	if _, p := Ship(shipped, "next"); p != nil {
		t.Fatalf("next shipping = %+v", p)
	}
	if _, p := Ship(line, "missing"); p == nil || p.Code != CodeNotOnLine {
		t.Fatalf("missing release = %+v", p)
	}
}
