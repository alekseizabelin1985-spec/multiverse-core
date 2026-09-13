package consumer

import (
	"context"
	"database/sql"
	"log/slog"
	"slices"

	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// OnCombat is the effect of combat.decided: the turn it answers has its
// mechanics, and every living player of the scope gets the text of the rules
// (kind=mechanics; component §8.1).
func (d *Deliveries) OnCombat(ctx context.Context, tx *sql.Tx, ev eventbus.Event, _ readmodel.Result) error {
	now := d.Clock.Now()
	if err := d.Turns.OnMechanics(ctx, tx, ev, now); err != nil {
		return err
	}
	text, data, ok := outbox.Mechanics(ev, d.Model)
	if !ok {
		d.Log.LogAttrs(ctx, slog.LevelWarn, "consumer: a decision without a text of the rules",
			slog.String("event_id", ev.ID), slog.Bool("handled", true))
		return nil
	}
	players, round := d.combatRecipients(ev)
	proto := outbox.Delivery{Kind: outbox.KindMechanics, GeneratedBy: outbox.GeneratedByRules, Text: text, Data: data, RoundSeq: round}
	return d.enqueue(ctx, tx, ev, players, proto, nil)
}

// combatRecipients are the living players of the scope of a decision — the
// player of a solo scope, the members of a group — and the number of the round
// of a group.
//
// The players the decision is about are recipients too, dead or not: a blow
// that killed its defender is still news to the one it killed.
func (d *Deliveries) combatRecipients(ev eventbus.Event) ([]string, *int) {
	var p struct {
		Round    struct{ Seq int } `json:"round"`
		Attacker named             `json:"attacker"`
		Defender *named            `json:"defender"`
	}
	_ = decode(ev, &p)
	involved := make([]string, 0, 2)
	for _, ref := range []*named{&p.Attacker, p.Defender} {
		if ref != nil && ref.Entity.Type == entity.TypePlayer && ref.Entity.ID != "" {
			involved = append(involved, ref.Entity.ID)
		}
	}
	scope := eventbus.GetScopeFromEvent(ev)
	switch {
	case scope != nil && scope.Type == entity.TypeGroup:
		players := d.membersOf(scope.ID)
		for _, id := range involved {
			if g, ok := d.Model.Group(scope.ID); ok && isMember(g, id) && !slices.Contains(players, id) {
				players = append(players, id)
			}
		}
		var round *int
		if p.Round.Seq > 0 {
			seq := p.Round.Seq
			round = &seq
		}
		return players, round
	case scope != nil && scope.ID != "":
		return []string{scope.ID}, nil
	default:
		return involved, nil
	}
}

func isMember(g readmodel.Group, playerID string) bool {
	for _, m := range g.Members {
		if m.PlayerID == playerID {
			return true
		}
	}
	return false
}
