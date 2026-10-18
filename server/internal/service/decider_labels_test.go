package service

import "testing"

func TestDeciderSquadLabelsAreUnique(t *testing.T) {
	const first = "aaaaaaaa-1111-4111-8111-111111111111"
	const second = "aaaaaaaa-2222-4222-8222-222222222222"
	const third = "bbbbbbbb-3333-4333-8333-333333333333"
	for _, names := range []map[string]string{
		{first: "Backend", second: "Backend", third: "Design"},
		{first: "Backend", second: "Backend", third: "Backend (aaaaaaaa-1)"},
		{first: "Backend", third: "Backend", second: "Backend (aaaaaaaa)"},
	} {
		rule := DeciderRule{Decision: "squad", Squads: make(map[string]deciderSquadOption)}
		counts := make(map[string]int)
		for id, name := range names {
			rule.Squads[id] = deciderSquadOption{Name: name}
			counts[name]++
		}
		labels := make(map[string]string)
		for id, name := range names {
			label := rule.choiceLabel(id)
			if other, exists := labels[label]; exists {
				t.Fatalf("squads %s and %s share answer %q", other, id, label)
			}
			labels[label] = id
			if counts[name] == 1 && label != name {
				t.Fatalf("unambiguous name %q became %q", name, label)
			}
			if again := rule.choiceLabel(id); again != label {
				t.Fatalf("label changed from %q to %q", label, again)
			}
		}
	}
}
