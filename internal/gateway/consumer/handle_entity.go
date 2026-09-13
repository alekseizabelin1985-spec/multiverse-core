package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// named is an EntityWithName of a payload.
type named struct {
	Entity eventbus.EntityRef `json:"entity"`
	Name   string             `json:"name"`
}

// OnUpdated is the effect of entity.updated (component §8.1). The turn of its
// correlation has its mechanics. A player who moved, rested or took loot hears
// it (kind=mechanics); a player in a group scope does not hear a move, because
// a group moves as one and the fact of the group tells every member. A player
// whose character died hears it (kind=system).
//
// The projection is read after the fact was applied to it: the health of a
// rest and the scope of a move are those the fact left.
func (d *Deliveries) OnUpdated(ctx context.Context, tx *sql.Tx, ev eventbus.Event, _ readmodel.Result) error {
	if err := d.Turns.OnMechanics(ctx, tx, ev, d.Clock.Now()); err != nil {
		return err
	}
	var p struct {
		Entity  named  `json:"entity"`
		Cause   string `json:"cause"`
		Changed []struct {
			Path string          `json:"path"`
			New  json.RawMessage `json:"new"`
		} `json:"changed"`
	}
	if err := decode(ev, &p); err != nil {
		return err
	}
	id := p.Entity.Entity.ID
	var players []string
	switch p.Entity.Entity.Type {
	case entity.TypePlayer:
		switch p.Cause {
		case outbox.CauseMove:
			if ch, ok := d.Model.Character(id); !ok || ch.Scope.Type != entity.TypeGroup {
				players = []string{id}
			}
		case outbox.CauseRest, outbox.CauseLoot:
			players = []string{id}
		}
		for _, c := range p.Changed {
			var status string
			if c.Path == entity.AttrStatus && json.Unmarshal(c.New, &status) == nil && status == entity.StatusDead {
				if err := d.enqueue(ctx, tx, ev, []string{id}, outbox.Delivery{Kind: outbox.KindSystem,
					GeneratedBy: outbox.GeneratedByRules, Text: outbox.Died()}, nil); err != nil {
					return err
				}
			}
		}
	case entity.TypeGroup:
		if p.Cause == outbox.CauseMove {
			players = d.membersOf(id)
		}
	}
	if len(players) == 0 {
		return nil
	}
	text, data, ok := outbox.Mechanics(ev, d.Model)
	if !ok {
		return nil
	}
	return d.enqueue(ctx, tx, ev, players, outbox.Delivery{Kind: outbox.KindMechanics, GeneratedBy: outbox.GeneratedByRules,
		Text: text, Data: data}, nil)
}

// OnRejected is the effect of entity.update.rejected on a proposal of the
// gateway: the move or the rest of a player that State refused reaches the
// player as kind=system, without the reason of State (acceptance of T-305).
//
// The player is found by the chain of the proposal. Its correlation is the
// action, the turn of the action names the player, and the proposal_id of the
// gateway is derived from the action and the character (actions.ProposalID). A
// refusal without a turn yet names the player by its entity, when it carries
// one. A refusal of another proposal of the same chain — the package of an
// encounter, a character being created — is not the gateway's and is passed
// over.
func (d *Deliveries) OnRejected(ctx context.Context, tx *sql.Tx, ev eventbus.Event, _ readmodel.Result) error {
	var p struct {
		ProposalID string `json:"proposal_id"`
		Entity     *named `json:"entity"`
	}
	if err := decode(ev, &p); err != nil {
		return err
	}
	action := ev.CorrelationID()
	if p.ProposalID == "" || action == "" || action == p.ProposalID {
		return nil
	}
	row, found, err := turns.Load(ctx, tx, action)
	if err != nil {
		return err
	}
	player, actionType := row.PlayerID, row.ActionType
	if !found && p.Entity != nil && p.Entity.Entity.Type == entity.TypePlayer {
		player = p.Entity.Entity.ID
	}
	if player == "" || actions.ProposalID(action, player) != p.ProposalID {
		return nil
	}
	if !found {
		d.Log.LogAttrs(ctx, slog.LevelWarn, "consumer: a refusal of the gateway before its turn was recorded",
			slog.String("event_id", ev.ID), slog.String("correlation_id", action), slog.Bool("handled", true))
	}
	return d.enqueue(ctx, tx, ev, []string{player}, outbox.Delivery{Kind: outbox.KindSystem,
		GeneratedBy: outbox.GeneratedByRules, Text: outbox.Refused(actionType)}, nil)
}

func decode(ev eventbus.Event, dst any) error {
	raw, err := json.Marshal(ev.Payload)
	if err == nil {
		err = json.Unmarshal(raw, dst)
	}
	if err != nil {
		return fmt.Errorf("consumer: payload of %s %s: %w", ev.Type, ev.ID, err)
	}
	return nil
}
