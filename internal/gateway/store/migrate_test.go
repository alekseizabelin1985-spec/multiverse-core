package store_test

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/store"
)

// These tests are unit tests although the tasks card of T-302 calls some of
// them integration: SQLite is an in-process library working on a temporary
// file, so they need neither Docker nor the network.

func TestMigrationsRunFromScratchAndRepeatAsNoOp(t *testing.T) {
	cases := []struct {
		name    string
		path    func(dir string) string
		open    func(ctx context.Context, path string) (*sql.DB, error)
		migrate func(ctx context.Context, db *sql.DB) (int, error)
	}{
		{"links.db", store.LinksPath, store.OpenLinks, store.MigrateLinks},
		{"gateway.db", store.GatewayPath, store.OpenGateway, store.MigrateGateway},
	}
	ctx := context.Background()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.path(tempDir(t))
			db, err := tc.open(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })

			if n, err := tc.migrate(ctx, db); err != nil || n != 1 {
				t.Fatalf("first migrate = %d, %v; want 1 migration applied", n, err)
			}
			if n, err := tc.migrate(ctx, db); err != nil || n != 0 {
				t.Fatalf("second migrate = %d, %v; want a no-op", n, err)
			}

			// A restart: the version is read from the file, not remembered.
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			again, err := tc.open(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = again.Close() })
			if n, err := tc.migrate(ctx, again); err != nil || n != 0 {
				t.Fatalf("migrate after reopen = %d, %v; want a no-op", n, err)
			}
			if v := queryInt(t, again, "SELECT MAX(version_id) FROM goose_db_version WHERE is_applied"); v != 1 {
				t.Errorf("schema version = %d, want 1", v)
			}
		})
	}
}

// linksColumns and gatewayColumns are the allow-lists of SEC-03. A new column
// fails this test on purpose: whoever adds it has to write it down here and
// answer whether it can carry an external ID.
var (
	linksColumns = map[string][]string{
		"character_requests": {"link_id", "action_key", "player_id", "status_code", "response_json", "expires_at"},
		"goose_db_version":   {"id", "version_id", "is_applied", "tstamp"},
		"links": {"link_id", "external_platform", "external_id", "player_id", "world_id", "status",
			"notice_shown_at", "consent_at", "age_confirmed_at", "last_seen_at", "created_at"},
	}
	gatewayColumns = map[string][]string{
		"cursors":             {"topic", "offset", "event_id", "updated_at"},
		"deliveries":          {"seq", "id", "world_id", "player_id", "platform", "kind", "correlation_id", "event_id", "round_seq", "generated_by", "fallback_reason", "text", "data", "state", "attempts", "leased_by", "leased_until", "created_at", "delivered_at", "expires_at"},
		"goose_db_version":    {"id", "version_id", "is_applied", "tstamp"},
		"group_participation": {"scope_id", "player_id", "missed_rounds", "participation"},
		"idempotency_keys":    {"player_id", "action_key", "correlation_id", "status_code", "response_json", "created_at", "expires_at"},
		"pending_characters":  {"proposal_id", "player_id", "world_id", "name", "actor_kind", "created_at", "deadline_at"},
		"processed_events":    {"event_id", "topic", "processed_at"},
		"rounds": {"scope_id", "seq", "world_id", "encounter_id", "state", "timeout_ms", "idle_after_missed", "expected", "acted", "auto_defended", "idle",
			"opened_at", "deadline_at", "closed_at", "close_reason", "opened_event_id", "closed_event_id", "narrative_event_id"},
		"sessions": {"id", "world_id", "scope_id", "scope_type", "kind", "actor_kind", "participants", "started_at", "last_action_at", "ended_at",
			"end_reason", "turns_count", "turns_degraded", "turns_failed", "state"},
		"turns": {"correlation_id", "session_id", "seq", "world_id", "scope_id", "scope_type", "player_id", "player_name", "action_type", "target_id", "target_type", "text_len",
			"round_seq", "status", "received_at", "acked_at", "mechanics_at", "narrative_at", "deadline_at", "gm_path", "phase1_mode", "lod",
			"generated_by", "agent_level", "agent_blueprint", "fallback_reason", "filter_applied", "recipients_count", "delivered_count",
			"result_event_id", "narrative_event_id", "absence", "reject_code"},
	}
)

func TestSchemaColumnsMatchTheAllowList(t *testing.T) {
	dir := tempDir(t)
	links := columns(t, openLinks(t, dir))
	gateway := columns(t, openGateway(t, dir))

	compareColumns(t, "links.db", links, linksColumns)
	compareColumns(t, "gateway.db", gateway, gatewayColumns)

	// SEC-03 independently of the allow-list above, so that editing the list
	// cannot let an external ID into gateway.db.
	for table, cols := range gateway {
		for _, col := range cols {
			if strings.Contains(strings.ToLower(col), "external") {
				t.Errorf("gateway.db %s.%s: a column named external* belongs only in links.db (SEC-03)", table, col)
			}
			if col == "link_id" {
				t.Errorf("gateway.db %s.link_id: link_id never leaves links.db (SEC-03)", table)
			}
		}
	}
	if !slices.Contains(links["links"], "external_id") {
		t.Error("links.db links has no external_id: the allow-list check above compares against the wrong schema")
	}
}

// columns lists every column of every user table, in declaration order.
func columns(t *testing.T, db *sql.DB) map[string][]string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), `
		SELECT m.name, p.name
		FROM sqlite_schema AS m, pragma_table_info(m.name) AS p
		WHERE m.type = 'table' AND substr(m.name, 1, 7) <> 'sqlite_'
		ORDER BY m.name, p.cid`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	got := map[string][]string{}
	for rows.Next() {
		var table, col string
		if err := rows.Scan(&table, &col); err != nil {
			t.Fatal(err)
		}
		got[table] = append(got[table], col)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return got
}

func compareColumns(t *testing.T, file string, got, want map[string][]string) {
	t.Helper()
	for table, cols := range got {
		allowed, ok := want[table]
		if !ok {
			t.Errorf("%s: table %s is not in the allow-list", file, table)
			continue
		}
		if !slices.Equal(cols, allowed) {
			t.Errorf("%s: table %s columns\n got  %v\n want %v", file, table, cols, allowed)
		}
	}
	for table := range want {
		if _, ok := got[table]; !ok {
			t.Errorf("%s: table %s is missing", file, table)
		}
	}
}

// C-10 v1.1: analytics.session.ended carries end_reason in leave, idle, death,
// error or forget, and the CHECK of sessions.end_reason is the same list.
func TestSessionEndReasonCheck(t *testing.T) {
	db := openGateway(t, tempDir(t))
	insert := func(id, reason string) error {
		_, err := db.ExecContext(context.Background(), `INSERT INTO sessions
			(id, world_id, scope_id, scope_type, kind, actor_kind, participants,
			 started_at, last_action_at, ended_at, end_reason, state)
			VALUES (?, 'world-dark-forest', ?, 'solo', 'solo', 'human', '["player-a"]',
			 '2026-09-13T10:00:00Z', '2026-09-13T10:05:00Z', '2026-09-13T10:06:00Z', ?, 'ended')`,
			id, "solo:"+id, reason)
		return err
	}
	for _, reason := range []string{"leave", "idle", "death", "error", "forget"} {
		if err := insert("s-"+reason, reason); err != nil {
			t.Errorf("end_reason %q rejected: %v", reason, err)
		}
	}
	err := insert("s-forget_all", "forget_all")
	if err == nil || !strings.Contains(err.Error(), "CHECK constraint failed") {
		t.Errorf("end_reason forget_all: err = %v, want a CHECK constraint failure", err)
	}
}

func TestLinksSchemaInvariants(t *testing.T) {
	ctx := context.Background()
	db := openLinks(t, tempDir(t))
	insertLink(t, db, "telegram", "100200300", "link-1", "player-1")
	exec(t, db, `INSERT INTO character_requests
		(link_id, action_key, player_id, status_code, response_json, expires_at)
		VALUES ('link-1', 'k-1', 'player-1', 201, '{"player_id":"player-1"}', '2026-09-14T10:00:00Z')`)

	refused := []struct {
		name  string
		query string
	}{
		{"one player_id per link", `INSERT INTO links (link_id, external_platform, external_id, player_id, status, last_seen_at, created_at)
			VALUES ('link-2', 'telegram', '400500600', 'player-1', 'pending_consent', 'x', 'x')`},
		{"link_id is unique", `INSERT INTO links (link_id, external_platform, external_id, status, last_seen_at, created_at)
			VALUES ('link-1', 'telegram', '400500600', 'pending_consent', 'x', 'x')`},
		{"one link per external ID", `INSERT INTO links (link_id, external_platform, external_id, status, last_seen_at, created_at)
			VALUES ('link-3', 'telegram', '100200300', 'pending_consent', 'x', 'x')`},
		{"known platforms only", `INSERT INTO links (link_id, external_platform, external_id, status, last_seen_at, created_at)
			VALUES ('link-4', 'discord', '1', 'pending_consent', 'x', 'x')`},
		{"known statuses only", `INSERT INTO links (link_id, external_platform, external_id, status, last_seen_at, created_at)
			VALUES ('link-5', 'telegram', '2', 'forgotten', 'x', 'x')`},
		{"a request needs its link", `INSERT INTO character_requests (link_id, action_key, player_id, status_code, response_json, expires_at)
			VALUES ('link-none', 'k', 'p', 201, '{}', 'x')`},
		{"response_json is JSON", `INSERT INTO character_requests (link_id, action_key, player_id, status_code, response_json, expires_at)
			VALUES ('link-1', 'k-2', 'player-1', 201, 'not json', 'x')`},
	}
	for _, tc := range refused {
		if _, err := db.ExecContext(ctx, tc.query); err == nil {
			t.Errorf("%s: the insert was accepted", tc.name)
		}
	}

	// A player without a character yet: several links may have no player_id.
	exec(t, db, `INSERT INTO links (link_id, external_platform, external_id, status, last_seen_at, created_at)
		VALUES ('link-6', 'ci', 'player-B', 'pending_consent', 'x', 'x'), ('link-7', 'ci', 'player-C', 'pending_consent', 'x', 'x')`)

	exec(t, db, "DELETE FROM links WHERE external_platform = 'telegram' AND external_id = '100200300'")
	if n := queryInt(t, db, "SELECT COUNT(*) FROM character_requests"); n != 0 {
		t.Errorf("character_requests after the link was deleted: %d rows, want 0 (ON DELETE CASCADE)", n)
	}
}

func TestGatewaySchemaInvariants(t *testing.T) {
	ctx := context.Background()
	db := openGateway(t, tempDir(t))
	session := func(id, scope, state string) string {
		return fmt.Sprintf(`INSERT INTO sessions (id, world_id, scope_id, scope_type, kind, actor_kind, participants, started_at, last_action_at, state)
			VALUES ('%s', 'w', '%s', 'group', 'group', 'ci', '[]', 'x', 'x', '%s')`, id, scope, state)
	}
	round := func(seq int, state string) string {
		return fmt.Sprintf(`INSERT INTO rounds (scope_id, seq, world_id, encounter_id, state, timeout_ms, idle_after_missed,
			expected, acted, auto_defended, idle, opened_at, deadline_at, opened_event_id)
			VALUES ('group-1', %d, 'w', 'enc-1', '%s', 60000, 2, '[]', '[]', '[]', '[]', 'x', 'x', 'e-%d')`, seq, state, seq)
	}
	delivery := func(id, data string) string {
		return fmt.Sprintf(`INSERT INTO deliveries (id, world_id, player_id, platform, kind, correlation_id, event_id,
			generated_by, text, data, state, created_at, expires_at)
			VALUES ('%s', 'w', 'player-a', 'ci', 'narrative', 'c', 'e', 'template', 't', %s, 'pending', 'x', 'x')`, id, data)
	}

	exec(t, db, session("s-1", "group-1", "active"))
	exec(t, db, session("s-0", "group-1", "ended"))
	exec(t, db, round(1, "closed"))
	exec(t, db, round(2, "open"))
	exec(t, db, delivery("d-1", "NULL"))
	exec(t, db, delivery("d-2", `'{"round_seq":2}'`))

	refused := []struct{ name, query string }{
		{"one active session per scope", session("s-2", "group-1", "active")},
		{"one open round per scope", round(3, "closing")},
		{"delivery data is JSON", delivery("d-3", "'{broken'")},
		{"delivery id is unique", delivery("d-1", "NULL")},
		{"a turn needs its session", `INSERT INTO turns (correlation_id, session_id, seq, world_id, scope_id, scope_type, player_id, player_name, action_type, status, received_at)
			VALUES ('c-1', 's-none', 1, 'w', 'solo:p', 'solo', 'p', 'n', 'look', 'received', 'x')`},
	}
	for _, tc := range refused {
		if _, err := db.ExecContext(ctx, tc.query); err == nil {
			t.Errorf("%s: the insert was accepted", tc.name)
		}
	}
	if seq := queryInt(t, db, "SELECT MAX(seq) FROM deliveries"); seq != 2 {
		t.Errorf("deliveries seq = %d, want 2 (AUTOINCREMENT in creation order)", seq)
	}
}
