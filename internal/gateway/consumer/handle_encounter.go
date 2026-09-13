package consumer

import (
	"context"
	"database/sql"

	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/eventbus"
)

// OnEncounterOpened is the effect of the event that opened an encounter —
// encounter.started or entity.created of the encounter, whichever the
// projection saw first (readmodel.Result): its participants get the
// world_event of the rules (component §8.1).
//
// The claim of the opening lives in memory, and after a restart the second
// event of the pair opens the encounter once more. The delivery is therefore
// named by the encounter and the transition, not by the event
// (outbox.TransitionDeliveryID): the second opening finds the row of the first
// in gateway.db and adds nothing (review #1 of T-304, Mi-2).
func (d *Deliveries) OnEncounterOpened(ctx context.Context, tx *sql.Tx, ev eventbus.Event, res readmodel.Result) error {
	if res.EncounterOpened == "" {
		return nil
	}
	enc, ok := d.Model.Encounter(res.EncounterOpened)
	if !ok {
		return nil
	}
	text, data := outbox.EncounterOpened(enc, d.Model)
	proto := outbox.Delivery{Kind: outbox.KindWorldEvent, GeneratedBy: outbox.GeneratedByRules, Text: text, Data: data}
	return d.enqueue(ctx, tx, ev, d.living(enc.Participants), proto, func(player string) string {
		return outbox.TransitionDeliveryID(enc.ID, outbox.TransitionOpened, player)
	})
}
