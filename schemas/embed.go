// Package schemas embeds the JSON Schema files of the event contract so that a
// binary validates without carrying the repository around (foundation.md §6,
// ADR-007).
//
// The tree is events/<type>.v<n>.json plus the two files every schema refers
// to: _common.json (shared $defs) and _envelope.json (the envelope itself).
// The all: prefix is required — a plain directory pattern would skip exactly
// those two files, because their names start with an underscore.
package schemas

import "embed"

// FS holds schemas/events/**. shared/contracts compiles it once at start.
//
//go:embed all:events
var FS embed.FS
