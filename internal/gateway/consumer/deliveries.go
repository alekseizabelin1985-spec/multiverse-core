package consumer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// TypeNarrativeOutput is the narrative of the swarm for the players (C-05).
const TypeNarrativeOutput = "narrative.output"

// Projection is what the deliveries read of the read model.
type Projection interface {
	outbox.Projection
	Group(id string) (readmodel.Group, bool)
}

// PlayerLinks is what the deliveries read of links.db: the link of a player,
// whose platform a delivery is enqueued for (component §8.1).
type PlayerLinks interface {
	ByPlayer(ctx context.Context, playerID string) (links.Link, bool, error)
}

// Deliveries are the effects of the consumer that feed the outbox and move the
// turns on (component §8.1, §7.6): the result of the mechanics, the facts a
// player hears about, the opening of an encounter, a refusal of State of a move
// or a rest, and the narrative.
type Deliveries struct {
	Model  Projection
	Outbox *outbox.Store
	Links  PlayerLinks
	Turns  *turns.Tracker
	Clock  clock.Clock
	Log    *slog.Logger
}

// Effects are the effects of d per event type, to be merged with the other
// effects of the consumer.
func (d *Deliveries) Effects() (map[string][]Effect, error) {
	if d.Model == nil || d.Outbox == nil || d.Links == nil || d.Turns == nil || d.Clock == nil {
		return nil, errors.New("consumer: Deliveries needs Model, Outbox, Links, Turns and Clock")
	}
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	return map[string][]Effect{
		outbox.TypeCombatDecided:       {d.OnCombat},
		readmodel.TypeEntityUpdated:    {d.OnUpdated},
		readmodel.TypeEntityCreated:    {d.OnEncounterOpened},
		readmodel.TypeEncounterStarted: {d.OnEncounterOpened},
		readmodel.TypeUpdateRejected:   {d.OnRejected},
		TypeNarrativeOutput:            {d.OnNarrative},
	}, nil
}

// MergeEffects joins the effects of several owners; the effects of one type
// run in the order the maps are given.
func MergeEffects(maps ...map[string][]Effect) map[string][]Effect {
	out := make(map[string][]Effect)
	for _, m := range maps {
		for typ, effects := range m {
			out[typ] = append(out[typ], effects...)
		}
	}
	return out
}

// recipient is a player a delivery goes to and the platform of its link now;
// no platform for a player without a link.
type recipient struct {
	player, platform string
}

// recipients looks up the link of every player, each player once, in order.
func (d *Deliveries) recipients(ctx context.Context, ev eventbus.Event, players []string) ([]recipient, error) {
	out := make([]recipient, 0, len(players))
	seen := make(map[string]struct{}, len(players))
	for _, player := range players {
		if _, dup := seen[player]; dup || player == "" {
			continue
		}
		seen[player] = struct{}{}
		link, found, err := d.Links.ByPlayer(ctx, player)
		if err != nil {
			return nil, fmt.Errorf("consumer: link of the recipient of %s: %w", ev.ID, err)
		}
		r := recipient{player: player}
		if found {
			r.platform = link.Platform
		}
		out = append(out, r)
	}
	return out, nil
}

// linked counts the recipients a delivery can reach: those with a link.
func linked(rs []recipient) int {
	n := 0
	for _, r := range rs {
		if r.platform != "" {
			n++
		}
	}
	return n
}

// enqueue writes one delivery of ev per player through tx (recipients). idOf
// names the delivery of a player; nil is outbox.EventDeliveryID.
func (d *Deliveries) enqueue(ctx context.Context, tx *sql.Tx, ev eventbus.Event, players []string, proto outbox.Delivery,
	idOf func(playerID string) string) error {
	rs, err := d.recipients(ctx, ev, players)
	if err != nil {
		return err
	}
	return d.write(ctx, tx, ev, rs, proto, idOf)
}

// write writes one delivery of ev per recipient through tx; a recipient
// without a platform is written dropped (outbox.Store.Enqueue).
func (d *Deliveries) write(ctx context.Context, tx *sql.Tx, ev eventbus.Event, rs []recipient, proto outbox.Delivery,
	idOf func(playerID string) string) error {
	if len(rs) == 0 {
		return nil
	}
	world := ""
	if ev.World != nil {
		world = ev.World.Entity.ID
	}
	ds := make([]outbox.Delivery, 0, len(rs))
	for _, r := range rs {
		delivery := proto
		delivery.WorldID, delivery.PlayerID, delivery.Platform = world, r.player, r.platform
		delivery.EventID, delivery.CorrelationID = ev.ID, ev.CorrelationID()
		if idOf != nil {
			delivery.ID = idOf(r.player)
		}
		ds = append(ds, delivery)
	}
	_, err := d.Outbox.Enqueue(ctx, tx, d.Clock.Now(), ds...)
	return err
}

// living keeps the players whose character the projection does not know to be
// dead or abandoned, each once, in order.
func (d *Deliveries) living(players []string) []string {
	out := make([]string, 0, len(players))
	for _, p := range players {
		if p == "" || slices.Contains(out, p) {
			continue
		}
		if ch, ok := d.Model.Character(p); ok && entity.IsTerminalStatus(ch.Status) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// membersOf are the living members of a group.
func (d *Deliveries) membersOf(groupID string) []string {
	g, ok := d.Model.Group(groupID)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(g.Members))
	for _, m := range g.Members {
		ids = append(ids, m.PlayerID)
	}
	return d.living(ids)
}
