// Package handlers serves the operations of the gateway HTTP API over the
// packages that own the data (component §5.2).
//
// The handlers are not in internal/gateway/api, where component §3 places
// them: api holds the wire types the Telegram bot imports, and a handler of
// links imports the SQLite store. Next to the DTOs, a handler would link the
// database driver into the bot, whose only platform imports are api and client
// (component §3, T-310).
package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/shared/clock"
)

// NoticeRepeatAfter is how long after the previous visit the notice is shown
// again (api-contracts.md §1.2).
const NoticeRepeatAfter = 30 * 24 * time.Hour

// ForgetRetryAfter is the Retry-After of 503 forget_incomplete, whole seconds.
// It equals the busy_timeout of links.db, which the blocked checkpoint has
// already spent waiting for its reader: a repeat sooner than that most likely
// meets the same reader.
const ForgetRetryAfter = 5 * time.Second

// LinkStore is what the handlers of links use of links.Store.
type LinkStore interface {
	Resolve(ctx context.Context, platform, externalID string, now time.Time) (links.Resolution, error)
	Consent(ctx context.Context, platform, externalID string, form links.ConsentForm, now time.Time) (links.Link, error)
	Forget(ctx context.Context, platform, externalID string) (links.ForgetResult, error)
}

// Character statuses of ResolveResponse (C-08 v1.1).
const (
	CharacterNone     = "none"
	CharacterCreating = "creating"
	CharacterAlive    = "alive"
	CharacterDead     = "dead"
)

// Links serves resolveLink, consentLink and forgetLink. The fields are set
// before the process server serves; see api.Config for why that is safe.
//
// None of these handlers logs: their operations are nolog, and the access log
// of the middleware writes request_id and code for them (SEC-01, SEC-02).
type Links struct {
	Store LinkStore
	Clock clock.Clock
	// CharacterStatus reports the status of an attached character from the
	// read model (T-304, T-306). Until it is set, a character the gateway has
	// not seen a fact of is creating: the state between the proposal and the
	// fact.
	CharacterStatus func(playerID string) string
}

// Resolve serves POST /v1/links/resolve.
func (h *Links) Resolve(w http.ResponseWriter, r *http.Request) {
	var req api.ResolveRequest
	if e := decodeAccount(r, &req, func() (string, string) { return req.ExternalPlatform, req.ExternalID }); e != nil {
		_ = api.WriteError(w, e)
		return
	}
	now := h.Clock.Now()
	res, err := h.Store.Resolve(r.Context(), req.ExternalPlatform, req.ExternalID, now)
	if err != nil {
		_ = api.WriteError(w, api.NewError(api.CodeInternal, nil))
		return
	}
	link := res.Link
	_ = api.WriteJSON(w, http.StatusOK, api.ResolveResponse{
		LinkStatus:      link.Status,
		PlayerID:        link.PlayerID,
		WorldID:         link.WorldID,
		CharacterStatus: h.characterStatus(link.PlayerID),
		NoticeDue: res.Created || link.Status == links.StatusPendingConsent ||
			now.Sub(res.PreviousSeenAt) >= NoticeRepeatAfter,
	})
}

func (h *Links) characterStatus(playerID *string) string {
	switch {
	case playerID == nil:
		return CharacterNone
	case h.CharacterStatus == nil:
		return CharacterCreating
	default:
		return h.CharacterStatus(*playerID)
	}
}

// Consent serves POST /v1/links/consent.
func (h *Links) Consent(w http.ResponseWriter, r *http.Request) {
	var req api.ConsentRequest
	if e := decodeAccount(r, &req, func() (string, string) { return req.ExternalPlatform, req.ExternalID }); e != nil {
		_ = api.WriteError(w, e)
		return
	}
	form := links.ConsentForm{NoticeShown: req.NoticeShown, Consent: req.Consent, AgeConfirmed: req.AgeConfirmed, ShownAt: req.ShownAt}
	if form.Complete() && req.ShownAt.IsZero() {
		_ = api.WriteError(w, api.NewError(api.CodeInvalidRequest, map[string]any{"field": "shown_at"}))
		return
	}
	link, err := h.Store.Consent(r.Context(), req.ExternalPlatform, req.ExternalID, form, h.Clock.Now())
	switch {
	case errors.Is(err, links.ErrConsentIncomplete):
		_ = api.WriteError(w, api.NewError(api.CodeConsentIncomplete, nil))
		return
	case err != nil:
		_ = api.WriteError(w, api.NewError(api.CodeInternal, nil))
		return
	}
	// The store keeps a consented link with its three timestamps; a row
	// without one (a fixture, a hand edit) is a broken invariant, not a panic.
	if link.ConsentAt == nil || link.AgeConfirmedAt == nil || link.NoticeShownAt == nil {
		_ = api.WriteError(w, api.NewError(api.CodeInternal, nil))
		return
	}
	_ = api.WriteJSON(w, http.StatusOK, api.ConsentResponse{
		LinkStatus:     link.Status,
		ConsentAt:      *link.ConsentAt,
		AgeConfirmedAt: *link.AgeConfirmedAt,
		NoticeShownAt:  *link.NoticeShownAt,
	})
}

// Forget serves DELETE /v1/links.
//
// 503 forget_incomplete means the link is deleted but the wipe of links.db is
// not confirmed, unlike bus_unavailable, which means nothing was done. While
// the compaction stays pending every /forget answers so; the client repeats
// after Retry-After, and a 200 {deleted: false} after a 503 is the end of the
// same forgetting (links.SQLite.Forget).
func (h *Links) Forget(w http.ResponseWriter, r *http.Request) {
	var req api.ForgetRequest
	if e := decodeAccount(r, &req, func() (string, string) { return req.ExternalPlatform, req.ExternalID }); e != nil {
		_ = api.WriteError(w, e)
		return
	}
	res, err := h.Store.Forget(r.Context(), req.ExternalPlatform, req.ExternalID)
	switch {
	case errors.Is(err, links.ErrCompactionPending):
		w.Header().Set(api.HeaderRetryAfter, strconv.Itoa(int(ForgetRetryAfter/time.Second)))
		_ = api.WriteError(w, api.NewError(api.CodeForgetIncomplete, nil))
		return
	case err != nil:
		_ = api.WriteError(w, api.NewError(api.CodeInternal, nil))
		return
	}
	_ = api.WriteJSON(w, http.StatusOK, api.ForgetResponse{Deleted: res.Deleted, PlayerIDDetached: res.PlayerIDDetached})
}

// decodeAccount decodes a body that names an external account and checks the
// account: the platform of the spec (telegram) and a non-empty id.
func decodeAccount(r *http.Request, v any, account func() (platform, id string)) *api.Error {
	if e := api.DecodeJSON(r, v); e != nil {
		return e
	}
	platform, id := account()
	switch {
	case platform != links.PlatformTelegram:
		return api.NewError(api.CodeInvalidRequest, map[string]any{"field": "external_platform"})
	case id == "":
		return api.NewError(api.CodeInvalidRequest, map[string]any{"field": "external_id"})
	}
	return nil
}
