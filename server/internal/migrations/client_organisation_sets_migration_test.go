package migrations

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestClientOrganisationSetsMigration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("integration test requires Postgres at DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	// Temporary tables shadow live tables, exercising the exact pre-743 schema
	// without downgrading the database shared with other test packages.
	if _, err := conn.Exec(ctx, `
		CREATE TEMP TABLE member (
			id integer PRIMARY KEY,
			role text NOT NULL,
			organisation_id uuid,
			CONSTRAINT member_client_organisation_check CHECK ((role = 'client') = (organisation_id IS NOT NULL))
		);
		CREATE TEMP TABLE workspace_invitation (
			id integer PRIMARY KEY,
			role text NOT NULL,
			status text NOT NULL DEFAULT 'pending',
			organisation_id uuid,
			CONSTRAINT workspace_invitation_client_organisation_check CHECK ((role = 'client') = (organisation_id IS NOT NULL))
		);
		INSERT INTO member VALUES (1, 'client', '00000000-0000-0000-0000-000000000001'), (2, 'owner', NULL);
		INSERT INTO workspace_invitation (id, role, organisation_id) VALUES
			(1, 'client', '00000000-0000-0000-0000-000000000002'), (2, 'member', NULL);
	`); err != nil {
		t.Fatal(err)
	}
	applyMigrationFile(t, ctx, conn.Conn(), "743_client_organisation_sets.up.sql")
	for _, tc := range []struct {
		table   string
		binding string
	}{
		{table: "member", binding: "00000000-0000-0000-0000-000000000001"},
		{table: "workspace_invitation", binding: "00000000-0000-0000-0000-000000000002"},
	} {
		t.Run(tc.table, func(t *testing.T) {
			var ids []string
			if err := conn.QueryRow(ctx, `SELECT organisation_ids::text[] FROM `+tc.table+` WHERE id=1`).Scan(&ids); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(ids, []string{tc.binding}) {
				t.Fatalf("pre-existing binding lost: %v", ids)
			}
			if err := conn.QueryRow(ctx, `SELECT organisation_ids::text[] FROM `+tc.table+` WHERE id=2`).Scan(&ids); err != nil {
				t.Fatal(err)
			}
			if ids == nil || len(ids) != 0 {
				t.Fatalf("staff binding = %v, want non-null empty array", ids)
			}
			var singularExists, pluralNotNull bool
			if err := conn.QueryRow(ctx, `SELECT EXISTS (
				SELECT 1 FROM pg_attribute WHERE attrelid = $1::regclass
				AND attname = 'organisation_id' AND NOT attisdropped
			), EXISTS (
				SELECT 1 FROM pg_attribute WHERE attrelid = $1::regclass
				AND attname = 'organisation_ids' AND attnotnull AND NOT attisdropped
			)`, "pg_temp."+tc.table).Scan(&singularExists, &pluralNotNull); err != nil {
				t.Fatal(err)
			}
			if singularExists || !pluralNotNull {
				t.Fatalf("wrong migrated columns: singular=%v pluralNotNull=%v", singularExists, pluralNotNull)
			}
			assertInsertCheckViolation(t, ctx, conn.Conn(), `INSERT INTO `+tc.table+` (id, role) VALUES (3, 'client')`)
			assertInsertCheckViolation(t, ctx, conn.Conn(), `INSERT INTO `+tc.table+` (id, role, organisation_ids) VALUES (3, 'member', ARRAY[$1::uuid])`, tc.binding)
			assertInsertCheckViolation(t, ctx, conn.Conn(), `INSERT INTO `+tc.table+` (id, role, organisation_ids) VALUES (3, 'client', ARRAY[NULL::uuid])`)
			if _, err := conn.Exec(ctx, `INSERT INTO `+tc.table+` (id, role) VALUES (3, 'member')`); err != nil {
				t.Fatalf("staff default rejected: %v", err)
			}
			if _, err := conn.Exec(ctx, `UPDATE `+tc.table+` SET organisation_ids=ARRAY[$1::uuid, '00000000-0000-0000-0000-000000000003'::uuid] WHERE id=1`, tc.binding); err != nil {
				t.Fatalf("multiple client organisations rejected: %v", err)
			}
		})
	}
	var status string
	if err := conn.QueryRow(ctx, `SELECT status FROM workspace_invitation WHERE id=1`).Scan(&status); err != nil || status != "pending" {
		t.Fatalf("migration changed pending invitation: status=%s err=%v", status, err)
	}
	applyMigrationFile(t, ctx, conn.Conn(), "743_client_organisation_sets.down.sql")
	for _, table := range []string{"member", "workspace_invitation"} {
		var binding string
		if err := conn.QueryRow(ctx, `SELECT organisation_id::text FROM `+table+` WHERE id=1`).Scan(&binding); err != nil {
			t.Fatal(err)
		}
		want := "00000000-0000-0000-0000-000000000001"
		if table == "workspace_invitation" {
			want = "00000000-0000-0000-0000-000000000002"
		}
		if binding != want {
			t.Fatalf("%s rollback binding=%s, want first UUID %s", table, binding, want)
		}
		var pluralExists bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid=$1::regclass AND attname='organisation_ids' AND NOT attisdropped)`, "pg_temp."+table).Scan(&pluralExists); err != nil {
			t.Fatal(err)
		}
		if pluralExists {
			t.Fatalf("%s rollback retained organisation_ids", table)
		}
		assertInsertCheckViolation(t, ctx, conn.Conn(), `INSERT INTO `+table+` (id, role) VALUES (4, 'client')`)
	}
}
