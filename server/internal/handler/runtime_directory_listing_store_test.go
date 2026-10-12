package handler

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestDirectoryListingStoreLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryDirectoryListingStore()
	first, err := store.Create(ctx, "runtime-a", "~/skills")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(ctx, "runtime-a", "/notes")
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.Create(ctx, "runtime-b", "")
	if err != nil {
		t.Fatal(err)
	}
	store.requests[first.ID].CreatedAt = time.Now().Add(-time.Second)
	if pending, err := store.HasPending(ctx, "runtime-a"); err != nil || !pending {
		t.Fatalf("has pending = %v, %v", pending, err)
	}
	for _, want := range []*RuntimeDirectoryListingRequest{first, second} {
		got, err := store.PopPending(ctx, "runtime-a")
		if err != nil || got == nil || got.ID != want.ID || got.Path != want.Path || got.Status != RuntimeLocalSkillRunning || got.RunStartedAt == nil {
			t.Fatalf("pop = %#v, %v; want %s", got, err, want.ID)
		}
	}
	if got, err := store.PopPending(ctx, "runtime-a"); err != nil || got != nil {
		t.Fatalf("duplicate pop = %#v, %v", got, err)
	}
	if pending, err := store.HasPending(ctx, "runtime-a"); err != nil || pending {
		t.Fatalf("pending after pop = %v, %v", pending, err)
	}
	if got, err := store.PopPending(ctx, "runtime-b"); err != nil || got == nil || got.ID != other.ID {
		t.Fatalf("other runtime pop = %#v, %v", got, err)
	}

	entries := []RuntimeDirectoryEntry{{Name: "SKILL.md", Path: "/skills/SKILL.md", Kind: "file"}, {Name: ".hidden", Path: "/skills/.hidden", Kind: "folder", Hidden: true}}
	if err := store.Complete(ctx, first.ID, "/skills", entries); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, first.ID)
	if err != nil || got.Status != RuntimeLocalSkillCompleted || got.Path != "/skills" || got.ResolvedPath != "/skills" || !reflect.DeepEqual(got.Entries, entries) {
		t.Fatalf("completion = %#v, %v", got, err)
	}
	entries[0].Name = "mutated input"
	got.Entries[0].Name = "mutated snapshot"
	if err := store.Fail(ctx, first.ID, "stale report"); err != nil {
		t.Fatal(err)
	}
	got, err = store.Get(ctx, first.ID)
	if err != nil || got.Status != RuntimeLocalSkillCompleted || got.Entries[0].Name != "SKILL.md" || got.Error != "" {
		t.Fatalf("terminal snapshot = %#v, %v", got, err)
	}
	if err := store.Fail(ctx, second.ID, "path does not exist: /notes"); err != nil {
		t.Fatal(err)
	}
	got, err = store.Get(ctx, second.ID)
	if err != nil || got.Status != RuntimeLocalSkillFailed || got.Error == "" {
		t.Fatalf("failure = %#v, %v", got, err)
	}
	if err := store.Complete(ctx, second.ID, "/notes", nil); err != nil {
		t.Fatal(err)
	}
	got, err = store.Get(ctx, second.ID)
	if err != nil || got.Status != RuntimeLocalSkillFailed {
		t.Fatalf("stale completion = %#v, %v", got, err)
	}
	if got, err := store.Get(ctx, "missing"); err != nil || got != nil {
		t.Fatalf("missing = %#v, %v", got, err)
	}
}

func TestDirectoryListingStoreTimeouts(t *testing.T) {
	for _, status := range []RuntimeLocalSkillRequestStatus{RuntimeLocalSkillPending, RuntimeLocalSkillRunning} {
		t.Run(string(status), func(t *testing.T) {
			ctx := context.Background()
			store := NewInMemoryDirectoryListingStore()
			req, err := store.Create(ctx, "runtime", "")
			if err != nil {
				t.Fatal(err)
			}
			stored := store.requests[req.ID]
			stored.Status = status
			stored.CreatedAt = time.Now().Add(-runtimeLocalSkillPendingTimeout - time.Second)
			started := time.Now().Add(-runtimeLocalSkillRunningTimeout - time.Second)
			stored.RunStartedAt = &started
			if pending, err := store.HasPending(ctx, "runtime"); err != nil || pending {
				t.Fatalf("expired pending = %v, %v", pending, err)
			}
			got, err := store.Get(ctx, req.ID)
			if err != nil || got.Status != RuntimeLocalSkillTimeout || got.Error == "" {
				t.Fatalf("timeout = %#v, %v", got, err)
			}
			if got, err := store.PopPending(ctx, "runtime"); err != nil || got != nil {
				t.Fatalf("expired pop = %#v, %v", got, err)
			}
			if err := store.Complete(ctx, req.ID, "/late", nil); err != nil {
				t.Fatal(err)
			}
			got, err = store.Get(ctx, req.ID)
			if err != nil || got.Status != RuntimeLocalSkillTimeout {
				t.Fatalf("late completion = %#v, %v", got, err)
			}
		})
	}
}

func TestDirectoryListingStoreRetentionAndEmptyEntries(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryDirectoryListingStore()
	old, err := store.Create(ctx, "runtime", "")
	if err != nil {
		t.Fatal(err)
	}
	store.requests[old.ID].CreatedAt = time.Now().Add(-runtimeLocalSkillStoreRetention - time.Second)
	fresh, err := store.Create(ctx, "runtime", "")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := store.Get(ctx, old.ID); err != nil || got != nil {
		t.Fatalf("retained expired request = %#v, %v", got, err)
	}
	if err := store.Complete(ctx, fresh.ID, "/empty", nil); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, fresh.ID)
	if err != nil || got.Entries == nil || len(got.Entries) != 0 {
		t.Fatalf("empty entries = %#v, %v", got, err)
	}
}
