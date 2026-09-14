package actions

import (
	"context"
	"errors"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/shared/eventbus"
)

// Turn is an action as the turns of a session count it.
type Turn struct {
	Scope    eventbus.ScopeRef
	WorldID  string
	PlayerID string
	// Name is the name of the character.
	Name string
	Type string
	// TargetID and TargetType name the entity the action aims at: the region
	// of enter and leave, the NPC of attack; empty for an action without one.
	TargetID, TargetType string
	// TextLen is the length of the text of say in characters, after the input
	// filter; the text itself is not a part of a turn.
	TextLen   int
	ActorKind string
	At        time.Time
}

// Turns is the tracker of the turns of the sessions (component §7.6,
// internal/gateway/turns). Begin names the turn an action is about to become:
// it opens the session of the scope when there is none and reserves the number
// of the turn, so that no other action of the scope gets it even when this
// batch fails to publish and is repeated later (§5.5); it stores no turn.
//
// Accepted records the turn of an action right before its player.* event is
// published, so that the facts and the narrative of the action, which the bus
// may deliver before the publication returns, find their turn (§5.5). A repeat
// for the same event records nothing more. Withdrawn takes the turn back when
// the bus did not acknowledge that event: an action the client is told to
// repeat leaves no turn behind. Acked records the moment the whole batch was
// acknowledged and the client answered 202. Rejected records an action refused
// by its preconditions.
type Turns interface {
	Begin(ctx context.Context, t Turn) (api.TurnRef, error)
	Accepted(ctx context.Context, t Turn, ref api.TurnRef, eventID string) error
	Withdrawn(ctx context.Context, ref api.TurnRef, eventID string) error
	Acked(ctx context.Context, eventID string, at time.Time) error
	Rejected(ctx context.Context, t Turn, code string) error
}

// ErrBusUnavailable wraps an error of Turns.Begin that comes from the bus: the
// start of a session could not be published. The action is answered 503
// bus_unavailable, like an action whose own event did not go out.
var ErrBusUnavailable = errors.New("actions: bus unavailable")
