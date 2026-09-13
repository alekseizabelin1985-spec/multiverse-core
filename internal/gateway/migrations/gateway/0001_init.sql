-- gateway.db: sessions, turns, rounds, idempotency, outbox and consumer
-- cursors (ADR-019, component gateway-and-bot.md section 4.2). The whole
-- schema of increments I1 and I2 is here, rounds and group_participation
-- included, so that I2 adds no migration.
--
-- No column holds an external messenger ID or anything derived from one, and
-- link_id is not stored either: player_id is enough here (SEC-03). The store
-- tests compare the column names with an allow-list.
--
-- The player status abandoned is not stored in the gateway: it is a fact of
-- State (C-02), and the gateway only proposes it on /forget.
--
-- Time is ISO-8601 UTC in TEXT; JSON is TEXT guarded by json_valid.

-- +goose Up
CREATE TABLE sessions (
  id             TEXT PRIMARY KEY,                  -- "{scope.id}:{started_at_unix}"
  world_id       TEXT NOT NULL,
  scope_id       TEXT NOT NULL,
  scope_type     TEXT NOT NULL CHECK (scope_type IN ('solo','group')),
  kind           TEXT NOT NULL CHECK (kind IN ('solo','group')),
  actor_kind     TEXT NOT NULL CHECK (actor_kind IN ('human','ci','sim')),
  participants   TEXT NOT NULL CHECK (json_valid(participants)),
  started_at     TEXT NOT NULL,
  last_action_at TEXT NOT NULL,
  ended_at       TEXT,
  -- forget closes the session of a player who ran /forget; it is not an error
  -- (C-10 v1.1, analytics.session.ended carries the same list).
  end_reason     TEXT CHECK (end_reason IN ('leave','idle','death','error','forget')),
  turns_count    INTEGER NOT NULL DEFAULT 0,
  turns_degraded INTEGER NOT NULL DEFAULT 0,
  turns_failed   INTEGER NOT NULL DEFAULT 0,
  state          TEXT NOT NULL CHECK (state IN ('active','ended'))
);

CREATE UNIQUE INDEX ux_sessions_active ON sessions(scope_id) WHERE state = 'active';
CREATE INDEX ix_sessions_idle ON sessions(state, last_action_at);

CREATE TABLE turns (
  correlation_id     TEXT PRIMARY KEY,              -- the id of the action event
  session_id         TEXT NOT NULL REFERENCES sessions(id),
  seq                INTEGER NOT NULL,
  world_id           TEXT NOT NULL,
  scope_id           TEXT NOT NULL,
  scope_type         TEXT NOT NULL,
  player_id          TEXT NOT NULL,
  player_name        TEXT NOT NULL,
  action_type        TEXT NOT NULL,
  target_id          TEXT,
  target_type        TEXT,
  text_len           INTEGER,
  round_seq          INTEGER,
  status             TEXT NOT NULL CHECK (status IN ('received','accepted','mechanics_applied','narrated','completed','rejected','timeout','error','degraded')),
  received_at        TEXT NOT NULL,
  acked_at           TEXT,
  mechanics_at       TEXT,
  narrative_at       TEXT,
  deadline_at        TEXT,
  gm_path            TEXT,
  phase1_mode        TEXT,
  lod                TEXT,
  generated_by       TEXT,
  agent_level        TEXT,
  agent_blueprint    TEXT,
  fallback_reason    TEXT,
  filter_applied     INTEGER,
  recipients_count   INTEGER,
  delivered_count    INTEGER,
  result_event_id    TEXT,
  narrative_event_id TEXT,
  absence            TEXT CHECK (absence IS NULL OR json_valid(absence)),
  reject_code        TEXT
);

CREATE UNIQUE INDEX ux_turns_session_seq ON turns(session_id, seq);
CREATE INDEX ix_turns_open ON turns(status, deadline_at);
CREATE INDEX ix_turns_round ON turns(scope_id, round_seq);

-- Rounds of a group scope (ADR-020); a row is written on every transition so
-- that the coordinator can restore open rounds after a restart.
CREATE TABLE rounds (
  scope_id           TEXT NOT NULL,
  seq                INTEGER NOT NULL,
  world_id           TEXT NOT NULL,
  encounter_id       TEXT NOT NULL,
  state              TEXT NOT NULL CHECK (state IN ('open','closing','closed')),
  -- From encounter.started.round, or the environment default: kept for Restore.
  timeout_ms         INTEGER NOT NULL,
  idle_after_missed  INTEGER NOT NULL,
  expected           TEXT NOT NULL CHECK (json_valid(expected)),       -- ["player-A", ...]
  acted              TEXT NOT NULL CHECK (json_valid(acted)),          -- [{"player_id","event_id","at"}]
  auto_defended      TEXT NOT NULL CHECK (json_valid(auto_defended)),
  idle               TEXT NOT NULL CHECK (json_valid(idle)),
  opened_at          TEXT NOT NULL,
  deadline_at        TEXT NOT NULL,
  closed_at          TEXT,
  close_reason       TEXT CHECK (close_reason IN ('all_acted','timeout','explicit')),
  opened_event_id    TEXT NOT NULL,
  closed_event_id    TEXT,
  narrative_event_id TEXT,
  PRIMARY KEY (scope_id, seq)
) WITHOUT ROWID;

CREATE UNIQUE INDEX ux_rounds_open ON rounds(scope_id) WHERE state IN ('open','closing');

-- Missed rounds per member; the gateway is the only proposer of participation
-- (data-model section 4).
CREATE TABLE group_participation (
  scope_id      TEXT NOT NULL,
  player_id     TEXT NOT NULL,
  missed_rounds INTEGER NOT NULL DEFAULT 0,
  participation TEXT NOT NULL CHECK (participation IN ('active','idle')),
  PRIMARY KEY (scope_id, player_id)
) WITHOUT ROWID;

CREATE TABLE idempotency_keys (
  player_id      TEXT NOT NULL,
  action_key     TEXT NOT NULL,
  correlation_id TEXT NOT NULL,
  status_code    INTEGER NOT NULL,
  response_json  TEXT NOT NULL CHECK (json_valid(response_json)),
  created_at     TEXT NOT NULL,
  expires_at     TEXT NOT NULL,
  PRIMARY KEY (player_id, action_key)
) WITHOUT ROWID;

CREATE INDEX ix_idem_exp ON idempotency_keys(expires_at);

-- Characters waiting for entity.created (component section 7.1).
CREATE TABLE pending_characters (
  proposal_id TEXT PRIMARY KEY,                     -- = correlation_id = id of entity.create.proposed
  player_id   TEXT NOT NULL UNIQUE,
  world_id    TEXT NOT NULL,
  name        TEXT NOT NULL,
  actor_kind  TEXT NOT NULL,
  created_at  TEXT NOT NULL,
  deadline_at TEXT NOT NULL
);

-- Outbox of deliveries (ADR-006). The route to the messenger is not stored:
-- it comes from links.db at the moment of delivery.
CREATE TABLE deliveries (
  seq             INTEGER PRIMARY KEY AUTOINCREMENT, -- strict creation order
  id              TEXT NOT NULL UNIQUE,              -- "d-<ULID>"
  world_id        TEXT NOT NULL,
  player_id       TEXT NOT NULL,
  platform        TEXT NOT NULL,                     -- from the link at enqueue time
  kind            TEXT NOT NULL CHECK (kind IN ('ack','mechanics','narrative','world_event','group','system')),
  correlation_id  TEXT NOT NULL,
  event_id        TEXT NOT NULL,
  round_seq       INTEGER,
  generated_by    TEXT NOT NULL CHECK (generated_by IN ('rules','llm','template')),
  fallback_reason TEXT,
  text            TEXT NOT NULL,
  data            TEXT CHECK (data IS NULL OR json_valid(data)),
  state           TEXT NOT NULL CHECK (state IN ('pending','delivered','dropped')),
  attempts        INTEGER NOT NULL DEFAULT 0,
  leased_by       TEXT,
  leased_until    TEXT,
  created_at      TEXT NOT NULL,
  delivered_at    TEXT,
  expires_at      TEXT NOT NULL
);

CREATE INDEX ix_deliveries_pending ON deliveries(platform, state, player_id, seq);
CREATE INDEX ix_deliveries_lease ON deliveries(leased_until) WHERE leased_until IS NOT NULL;
CREATE INDEX ix_deliveries_exp ON deliveries(state, expires_at);
CREATE INDEX ix_deliveries_event ON deliveries(event_id, player_id);

CREATE TABLE processed_events (
  event_id     TEXT PRIMARY KEY,
  topic        TEXT NOT NULL,
  processed_at TEXT NOT NULL
);

CREATE INDEX ix_processed_at ON processed_events(processed_at);

CREATE TABLE cursors (
  topic      TEXT PRIMARY KEY,
  offset     INTEGER NOT NULL,
  event_id   TEXT,
  updated_at TEXT NOT NULL
);
