package consumer

import (
	"context"
	"database/sql"

	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// OnNarrative is the effect of narrative.output: the turn of its correlation
// has its narrative, and every recipient gets the text as kind=narrative with
// data{narrative_event_id, kind, absence?, filter} (component §8.1, C-05).
//
// The turn counts as its recipients the deliveries an acknowledgement can
// complete: each player of recipients[] once, and only one with a link. A
// recipient without a link gets a delivery written dropped, which nobody
// acknowledges, so a turn whose recipients have no link at all completes at
// once (review #1 of T-307, Ma-1; decision 3 of the orchestrator).
//
// The gateway reorders nothing: the delivery joins the queue of the player in
// the order the topic delivered the narrative, whatever its kind, based_on or
// time say (C-05 v1.4 p. 8). The order of a death after the text of its turn
// is the publisher's.
func (d *Deliveries) OnNarrative(ctx context.Context, tx *sql.Tx, ev eventbus.Event, _ readmodel.Result) error {
	var p struct {
		Recipients       []named        `json:"recipients"`
		Text             string         `json:"text"`
		GeneratedBy      string         `json:"generated_by"`
		FallbackReason   *string        `json:"fallback_reason"`
		Kind             string         `json:"kind"`
		NarrativeEventID string         `json:"narrative_event_id"`
		Absence          map[string]any `json:"absence"`
		Filter           map[string]any `json:"filter"`
		Round            *struct {
			Seq int `json:"seq"`
		} `json:"round"`
	}
	if err := decode(ev, &p); err != nil {
		return err
	}
	players := make([]string, 0, len(p.Recipients))
	for _, r := range p.Recipients {
		if r.Entity.Type == entity.TypePlayer && r.Entity.ID != "" {
			players = append(players, r.Entity.ID)
		}
	}
	rs, err := d.recipients(ctx, ev, players)
	if err != nil {
		return err
	}
	if err := d.Turns.OnNarrative(ctx, tx, ev, linked(rs)); err != nil {
		return err
	}
	narrativeID := p.NarrativeEventID
	if narrativeID == "" {
		narrativeID = ev.ID
	}
	data := map[string]any{outbox.DataNarrativeEventID: narrativeID, outbox.DataKind: p.Kind}
	if p.Absence != nil {
		data[outbox.DataAbsence] = p.Absence
	}
	if p.Filter != nil {
		data[outbox.DataFilter] = p.Filter
	}
	proto := outbox.Delivery{Kind: outbox.KindNarrative, GeneratedBy: p.GeneratedBy, FallbackReason: p.FallbackReason,
		Text: p.Text, Data: data}
	if p.Round != nil && p.Round.Seq > 0 {
		seq := p.Round.Seq
		proto.RoundSeq = &seq
	}
	return d.write(ctx, tx, ev, rs, proto, nil)
}
