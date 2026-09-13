-- links.db: the only place an external messenger ID is stored (ADR-009 p. 2,
-- ADR-019, component gateway-and-bot.md section 4.1). The file is opened with
-- secure_delete=ON and auto_vacuum=INCREMENTAL, and right after the DELETE
-- /forget vacuums and then checkpoints the WAL (store.CompactLinks), so a
-- forgotten ID is left neither in free pages nor in the WAL (SEC-04, SEC-05).
--
-- Username, profile name and chat_id are not stored: in a private Telegram
-- chat chat_id equals the user id, and the bot sends by external_id.

-- +goose Up
CREATE TABLE links (
  -- Surrogate of the link: a ULID assigned by the first resolve and never
  -- changed. It is the key of character_requests, so no external ID ends up in
  -- a key, and it never leaves links.db (SEC-03, C-08 v1.1).
  link_id           TEXT NOT NULL UNIQUE,
  external_platform TEXT NOT NULL CHECK (external_platform IN ('telegram','ci','sim')),
  -- Telegram user id as a string; the fixture name for ci and sim.
  external_id       TEXT NOT NULL,
  -- The current living (or being created) character. A dead character gets a
  -- new player_id on the same link.
  player_id         TEXT,
  world_id          TEXT,
  -- consented implies the three timestamps below are set. The code checks it
  -- in Consent: a CHECK cannot express a conditional NOT NULL without a
  -- trigger, and the schema has no triggers on purpose.
  status            TEXT NOT NULL CHECK (status IN ('pending_consent','consented')),
  notice_shown_at   TEXT,
  consent_at        TEXT,
  age_confirmed_at  TEXT,
  last_seen_at      TEXT NOT NULL,
  created_at        TEXT NOT NULL,
  PRIMARY KEY (external_platform, external_id)
) WITHOUT ROWID;

CREATE UNIQUE INDEX ux_links_player ON links(player_id) WHERE player_id IS NOT NULL;

-- Idempotency of POST /v1/characters. The table lives in links.db and not in
-- gateway.db because it goes together with the link on /forget (the cascade).
CREATE TABLE character_requests (
  link_id       TEXT NOT NULL REFERENCES links(link_id) ON DELETE CASCADE,
  action_key    TEXT NOT NULL,
  player_id     TEXT NOT NULL,
  status_code   INTEGER NOT NULL,
  response_json TEXT NOT NULL CHECK (json_valid(response_json)),
  expires_at    TEXT NOT NULL,
  PRIMARY KEY (link_id, action_key)
) WITHOUT ROWID;

CREATE INDEX ix_character_requests_exp ON character_requests(expires_at);
