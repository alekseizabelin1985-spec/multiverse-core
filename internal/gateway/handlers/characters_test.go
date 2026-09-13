package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/characters"
	"multiverse-core.io/internal/gateway/handlers"
	"multiverse-core.io/internal/gateway/readmodel"
)

type fakeCharacters struct {
	got    characters.Request
	calls  int
	answer characters.Answer
	state  *api.CharacterState
	err    *api.Error
	player string
}

func (f *fakeCharacters) Create(_ context.Context, r characters.Request) characters.Answer {
	f.got, f.calls = r, f.calls+1
	return f.answer
}

func (f *fakeCharacters) Player(_ context.Context, playerID string) (*api.CharacterState, *api.Error) {
	f.player = playerID
	return f.state, f.err
}

type fakeWorlds struct {
	worlds  []readmodel.World
	regions map[string][]readmodel.Region
}

func (f fakeWorlds) Worlds() []readmodel.World                 { return f.worlds }
func (f fakeWorlds) Regions(worldID string) []readmodel.Region { return f.regions[worldID] }

func serve(pattern string, h http.HandlerFunc, method, target, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc(pattern, h)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader(body)))
	return rec
}

// createCharacter hands the service the fields of the body and writes what it
// answered; a body that is no request never reaches the service.
func TestCreateCharacterPassesTheRequestAndWritesTheAnswer(t *testing.T) {
	created := true
	svc := &fakeCharacters{answer: characters.Answer{Status: http.StatusCreated,
		Body: &api.CreateCharacterResponse{PlayerID: "player-1", Created: &created}}}
	h := &handlers.Characters{Service: svc}
	rec := serve("POST /v1/characters", h.Create, http.MethodPost, "/v1/characters",
		`{"external_platform":"telegram","external_id":"`+externalID+`","world_id":"w","character_name":"Вася","action_key":"k"}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"player_id":"player-1"`) {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	if g := svc.got; g.Platform != "telegram" || g.ExternalID != externalID || g.WorldID != "w" || g.Name != "Вася" || g.ActionKey != "k" {
		t.Errorf("request = %+v", g)
	}

	svc = &fakeCharacters{answer: characters.Answer{Status: http.StatusNotFound, Err: api.NewError(api.CodeWorldNotFound, nil)}}
	h.Service = svc
	rec = serve("POST /v1/characters", h.Create, http.MethodPost, "/v1/characters", `{"world_id":"x"}`)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), api.CodeWorldNotFound) {
		t.Errorf("refusal = %d %s", rec.Code, rec.Body)
	}
	rec = serve("POST /v1/characters", h.Create, http.MethodPost, "/v1/characters", `{"world_id":`)
	if rec.Code != http.StatusBadRequest || svc.calls != 1 {
		t.Errorf("broken body = %d, service called %d times", rec.Code, svc.calls)
	}
}

func TestGetPlayerWritesTheStateOrTheError(t *testing.T) {
	svc := &fakeCharacters{state: &api.CharacterState{PlayerID: "player-1", Name: "Вася", WorldID: "w", Status: "creating"}}
	h := &handlers.Characters{Service: svc}
	rec := serve("GET /v1/players/{player_id}", h.Player, http.MethodGet, "/v1/players/player-1", "")
	if rec.Code != http.StatusOK || svc.player != "player-1" || !strings.Contains(rec.Body.String(), `"status":"creating"`) ||
		strings.Contains(rec.Body.String(), `"hp"`) {
		t.Errorf("player = %d %s", rec.Code, rec.Body)
	}
	h.Service = &fakeCharacters{err: api.NewError(api.CodePlayerNotFound, nil)}
	rec = serve("GET /v1/players/{player_id}", h.Player, http.MethodGet, "/v1/players/player-2", "")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), api.CodePlayerNotFound) {
		t.Errorf("unknown = %d %s", rec.Code, rec.Body)
	}
}

// listWorlds lists every world with its regions and the flag of the cloud off;
// no world is an empty list, not null.
func TestListWorldsListsTheWorldsWithTheirRegions(t *testing.T) {
	h := &handlers.Characters{Worlds: fakeWorlds{}}
	rec := serve("GET /v1/worlds", h.ListWorlds, http.MethodGet, "/v1/worlds", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"worlds":[]}` {
		t.Errorf("no worlds = %d %s", rec.Code, rec.Body)
	}
	h.Worlds = fakeWorlds{
		worlds:  []readmodel.World{{ID: "w", Name: "Мир", LawsVersion: "2"}, {ID: "empty", Name: "Пусто"}},
		regions: map[string][]readmodel.Region{"w": {{ID: "r-1", Name: "Опушка"}}},
	}
	rec = serve("GET /v1/worlds", h.ListWorlds, http.MethodGet, "/v1/worlds", "")
	want := `{"worlds":[{"world_id":"w","name":"Мир","regions":[{"region_id":"r-1","name":"Опушка"}],"laws_version":"2","llm":{"cloud_enabled":false}},` +
		`{"world_id":"empty","name":"Пусто","regions":[],"laws_version":"","llm":{"cloud_enabled":false}}]}`
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
		t.Errorf("worlds = %d %s", rec.Code, rec.Body)
	}
}
