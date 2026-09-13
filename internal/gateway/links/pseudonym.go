package links

import "errors"

// PlayerIDPrefix starts every player_id the gateway assigns.
const PlayerIDPrefix = "player-"

// IDSource returns a new unique identifier. The gateway context passes
// runtime.Deps.IDs, so --id-source=sequence makes player_id and link_id
// reproducible (ADR-010 p. 3); the design asked for ULIDs, which would need a
// dependency the module does not have (T-303, component §6).
type IDSource func() string

var errNoID = errors.New("links: the id source returned an empty id")

// NewPlayerID returns a pseudonymous player_id: it carries nothing of the
// external account (FR-060).
func NewPlayerID(ids IDSource) (string, error) {
	id, err := next(ids)
	if err != nil {
		return "", err
	}
	return PlayerIDPrefix + id, nil
}

// NewLinkID returns the surrogate key of a link; it never leaves links.db
// (SEC-03, C-08 v1.1).
func NewLinkID(ids IDSource) (string, error) { return next(ids) }

func next(ids IDSource) (string, error) {
	if ids == nil {
		return "", errors.New("links: no id source")
	}
	id := ids()
	if id == "" {
		return "", errNoID
	}
	return id, nil
}
