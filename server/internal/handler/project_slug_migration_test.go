package handler

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// TestProjectSlugMigrationBackfill runs migration 809's backfill against
// projects without a slug: each gets the kebab-case of its title, deduplicated
// per organisation with -2, -3, ... in creation order and past slugs already
// taken. The migration runs inside a rolled-back transaction.
func TestProjectSlugMigrationBackfill(t *testing.T) {
	if testPool == nil {
		t.Skip("handler test fixture not initialized (no DB?)")
	}
	ctx := context.Background()
	orgID, _ := slugTestOrganisation(t, "Backfill org")
	otherOrgID, _ := slugTestOrganisation(t, "Other backfill org")
	project := func(title, organisation string, minutesAgo int, slug any) string {
		return dbfx.Project(t, title, testutil.Cols{
			"organisation_id": organisation, "slug": slug,
			"created_at": testutil.Raw(fmt.Sprintf("now() - interval '%d minutes'", minutesAgo)),
		})
	}
	cases := []struct {
		id   string
		want string
	}{
		{project("Fan App!", orgID, 9, nil), "fan-app"},
		{project("  fan   app  ", orgID, 8, nil), "fan-app-2"},
		{project("Taken", orgID, 7, "fan-app-3"), "fan-app-3"},
		{project("FAN_APP", orgID, 6, nil), "fan-app-4"},
		{project("Back Office v2.0", orgID, 5, nil), "back-office-v2-0"},
		{project("***", orgID, 4, nil), "project"},
		{project("Fan App", otherOrgID, 3, nil), "fan-app"},
	}

	migration, err := os.ReadFile("../../migrations/809_project_slug.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("migration: %v", err)
	}
	for _, tc := range cases {
		var got string
		if err := tx.QueryRow(ctx, `SELECT slug FROM project WHERE id = $1`, tc.id).Scan(&got); err != nil {
			t.Fatalf("project %s: %v", tc.id, err)
		}
		if got != tc.want {
			t.Errorf("project %s slug = %q, want %q", tc.id, got, tc.want)
		}
	}
}
