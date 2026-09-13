package handlers

import (
	"context"
	"net/http"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/characters"
	"multiverse-core.io/internal/gateway/readmodel"
)

// CharacterService is what the handlers of characters use of
// characters.Service.
type CharacterService interface {
	Create(ctx context.Context, r characters.Request) characters.Answer
	Player(ctx context.Context, playerID string) (*api.CharacterState, *api.Error)
}

// WorldList is what listWorlds reads of the read model.
type WorldList interface {
	Worlds() []readmodel.World
	Regions(worldID string) []readmodel.Region
}

// Characters serves createCharacter, getPlayer and listWorlds. Its fields are
// set before the process server serves; see api.Config for why that is safe.
//
// Create does not log: its operation is nolog, and the body names the external
// account and the name of the character (SEC-01, SEC-02).
type Characters struct {
	Service CharacterService
	Worlds  WorldList
}

// Create serves POST /v1/characters.
func (h *Characters) Create(w http.ResponseWriter, r *http.Request) {
	var req api.CreateCharacterRequest
	if e := api.DecodeJSON(r, &req); e != nil {
		_ = api.WriteError(w, e)
		return
	}
	answer := h.Service.Create(r.Context(), characters.Request{
		Platform: req.ExternalPlatform, ExternalID: req.ExternalID, WorldID: req.WorldID,
		Name: req.CharacterName, ActionKey: req.ActionKey, ActorKind: api.ActorKindFrom(r.Context()),
	})
	if answer.Err != nil {
		_ = api.WriteError(w, answer.Err)
		return
	}
	_ = api.WriteJSON(w, answer.Status, answer.Body)
}

// Player serves GET /v1/players/{player_id}.
func (h *Characters) Player(w http.ResponseWriter, r *http.Request) {
	state, e := h.Service.Player(r.Context(), r.PathValue("player_id"))
	if e != nil {
		_ = api.WriteError(w, e)
		return
	}
	_ = api.WriteJSON(w, http.StatusOK, state)
}

// ListWorlds serves GET /v1/worlds: every world of the projection with its
// regions. llm.cloud_enabled stays false until the gateway projects
// config.cloud_enabled (T-320).
func (h *Characters) ListWorlds(w http.ResponseWriter, _ *http.Request) {
	out := api.WorldsResponse{Worlds: []api.WorldSummary{}}
	for _, world := range h.Worlds.Worlds() {
		summary := api.WorldSummary{WorldID: world.ID, Name: world.Name, LawsVersion: world.LawsVersion, Regions: []api.RegionSummary{}}
		for _, region := range h.Worlds.Regions(world.ID) {
			summary.Regions = append(summary.Regions, api.RegionSummary{RegionID: region.ID, Name: region.Name})
		}
		out.Worlds = append(out.Worlds, summary)
	}
	_ = api.WriteJSON(w, http.StatusOK, out)
}
