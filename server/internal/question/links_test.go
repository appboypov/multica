package question

import "testing"

func TestValidLinkType(t *testing.T) {
	for _, tc := range []struct {
		linkType string
		want     bool
	}{
		{LinkTypeIssue, true},
		{LinkTypeTask, true},
		{LinkTypeChat, true},
		{"run", false},
		{"chat_session", false},
		{"", false},
	} {
		t.Run(tc.linkType, func(t *testing.T) {
			if got := ValidLinkType(tc.linkType); got != tc.want {
				t.Fatalf("ValidLinkType(%q) = %v, want %v", tc.linkType, got, tc.want)
			}
		})
	}
}

func TestParseLinkRef(t *testing.T) {
	for _, tc := range []struct {
		value     string
		want      LinkRef
		wantError bool
	}{
		{"WORK-412", LinkRef{Type: LinkTypeIssue, Ref: "WORK-412"}, false},
		{"run:7f3a", LinkRef{Type: LinkTypeTask, Ref: "7f3a"}, false},
		{"chat:7f3a", LinkRef{Type: LinkTypeChat, Ref: "7f3a"}, false},
		{"chat: ", LinkRef{}, true},
		{"task:7f3a", LinkRef{}, true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			got, err := ParseLinkRef(tc.value)
			if (err != nil) != tc.wantError {
				t.Fatalf("ParseLinkRef(%q) error = %v, want error %v", tc.value, err, tc.wantError)
			}
			if got != tc.want {
				t.Fatalf("ParseLinkRef(%q) = %+v, want %+v", tc.value, got, tc.want)
			}
		})
	}
}
