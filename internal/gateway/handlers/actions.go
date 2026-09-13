package handlers

import (
	"context"
	"net/http"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
)

// ActionService is what the handler of actions uses of actions.Service.
type ActionService interface {
	Submit(ctx context.Context, c actions.Command) actions.Answer
}

// Actions serves postAction. Its field is set before the process server
// serves; see api.Config for why that is safe.
//
// The handler does not log: the text of say is the text of a player, and the
// access log of the middleware writes the route, the status and the code.
type Actions struct {
	Service ActionService
}

// Post serves POST /v1/players/{player_id}/actions.
func (h *Actions) Post(w http.ResponseWriter, r *http.Request) {
	var req api.ActionRequest
	if e := api.DecodeJSON(r, &req); e != nil {
		_ = api.WriteError(w, e)
		return
	}
	c := actions.Command{
		PlayerID:  r.PathValue("player_id"),
		ActionKey: req.ActionKey,
		Type:      req.Type,
		Text:      req.Text,
		ActorKind: api.ActorKindFrom(r.Context()),
	}
	if req.Target != nil {
		c.Target = *req.Target
	}
	answer := h.Service.Submit(r.Context(), c)
	if answer.Err != nil {
		_ = api.WriteError(w, answer.Err)
		return
	}
	_ = api.WriteJSON(w, answer.Status, answer.Accepted)
}
