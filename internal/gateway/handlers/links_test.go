package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/handlers"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/shared/clock"
)

const externalID = "7391846205"

var t0 = time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

// fakeStore answers with what a test puts in it.
type fakeStore struct {
	resolution links.Resolution
	link       links.Link
	forget     links.ForgetResult
	err        error
	gotForm    links.ConsentForm
}

func (f *fakeStore) Resolve(context.Context, string, string, time.Time) (links.Resolution, error) {
	return f.resolution, f.err
}

func (f *fakeStore) Consent(_ context.Context, _, _ string, form links.ConsentForm, _ time.Time) (links.Link, error) {
	f.gotForm = form
	return f.link, f.err
}

func (f *fakeStore) Forget(context.Context, string, string) (links.ForgetResult, error) {
	return f.forget, f.err
}

func call(t *testing.T, h http.HandlerFunc, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("answer %q: %v", rec.Body.String(), err)
	}
	return rec.Code, m
}

func code(m map[string]any) string {
	e, _ := m["error"].(map[string]any)
	c, _ := e["code"].(string)
	return c
}

func account(extra string) string {
	return `{"external_platform":"telegram","external_id":"` + externalID + `"` + extra + `}`
}

func ptr(s string) *string { return &s }

func TestResolveReportsTheCharacterAndTheNotice(t *testing.T) {
	now := clock.NewManual(t0.Add(handlers.NoticeRepeatAfter))
	for name, tc := range map[string]struct {
		res       links.Resolution
		status    func(string) string
		character string
		notice    bool
	}{
		"new link": {links.Resolution{Created: true, Link: links.Link{Status: links.StatusPendingConsent}, PreviousSeenAt: now.Now()},
			nil, "none", true},
		"pending": {links.Resolution{Link: links.Link{Status: links.StatusPendingConsent}, PreviousSeenAt: now.Now()},
			nil, "none", true},
		"seen a minute ago": {links.Resolution{Link: links.Link{Status: links.StatusConsented}, PreviousSeenAt: now.Now().Add(-time.Minute)},
			nil, "none", false},
		"seen 30 days ago": {links.Resolution{Link: links.Link{Status: links.StatusConsented}, PreviousSeenAt: t0},
			nil, "none", true},
		"character without a read model": {links.Resolution{Link: links.Link{Status: links.StatusConsented, PlayerID: ptr("player-A")}, PreviousSeenAt: now.Now()},
			nil, "creating", false},
		"character in the read model": {links.Resolution{Link: links.Link{Status: links.StatusConsented, PlayerID: ptr("player-A")}, PreviousSeenAt: now.Now()},
			func(id string) string { return map[string]string{"player-A": "dead"}[id] }, "dead", false},
	} {
		t.Run(name, func(t *testing.T) {
			h := &handlers.Links{Store: &fakeStore{resolution: tc.res}, Clock: now, CharacterStatus: tc.status}
			status, body := call(t, h.Resolve, account(""))
			if status != 200 || body["character_status"] != tc.character || body["notice_due"] != tc.notice {
				t.Errorf("= %d %v, want character %s notice %v", status, body, tc.character, tc.notice)
			}
			if _, has := body["link_id"]; has {
				t.Error("the answer carries link_id")
			}
		})
	}
}

func TestLinkHandlersMapStoreErrors(t *testing.T) {
	now := clock.NewManual(t0)
	storeDown := errors.New("disk I/O error")
	for name, tc := range map[string]struct {
		store   *fakeStore
		handler func(h *handlers.Links) http.HandlerFunc
		body    string
		status  int
		code    string
	}{
		"resolve fails":            {&fakeStore{err: storeDown}, func(h *handlers.Links) http.HandlerFunc { return h.Resolve }, account(""), 500, api.CodeInternal},
		"resolve of another app":   {&fakeStore{}, func(h *handlers.Links) http.HandlerFunc { return h.Resolve }, `{"external_platform":"ci","external_id":"x"}`, 400, api.CodeInvalidRequest},
		"resolve without an id":    {&fakeStore{}, func(h *handlers.Links) http.HandlerFunc { return h.Resolve }, `{"external_platform":"telegram"}`, 400, api.CodeInvalidRequest},
		"consent incomplete":       {&fakeStore{err: fmt.Errorf("x: %w", links.ErrConsentIncomplete)}, func(h *handlers.Links) http.HandlerFunc { return h.Consent }, account(`,"consent":true`), 400, api.CodeConsentIncomplete},
		"consent without shown":    {&fakeStore{}, func(h *handlers.Links) http.HandlerFunc { return h.Consent }, account(`,"notice_shown":true,"consent":true,"age_confirmed":true`), 400, api.CodeInvalidRequest},
		"consent fails":            {&fakeStore{err: storeDown}, func(h *handlers.Links) http.HandlerFunc { return h.Consent }, account(`,"consent":true`), 500, api.CodeInternal},
		"forget not yet wiped":     {&fakeStore{forget: links.ForgetResult{Deleted: true}, err: fmt.Errorf("%w: busy", links.ErrCompactionPending)}, func(h *handlers.Links) http.HandlerFunc { return h.Forget }, account(""), 503, api.CodeForgetIncomplete},
		"forget hook fails":        {&fakeStore{err: storeDown}, func(h *handlers.Links) http.HandlerFunc { return h.Forget }, account(""), 500, api.CodeInternal},
		"forget of another app":    {&fakeStore{}, func(h *handlers.Links) http.HandlerFunc { return h.Forget }, `{"external_platform":"sim","external_id":"x"}`, 400, api.CodeInvalidRequest},
		"consent of a broken body": {&fakeStore{}, func(h *handlers.Links) http.HandlerFunc { return h.Consent }, `{"external_platform":`, 400, api.CodeInvalidRequest},
	} {
		t.Run(name, func(t *testing.T) {
			h := &handlers.Links{Store: tc.store, Clock: now}
			status, body := call(t, tc.handler(h), tc.body)
			if status != tc.status || code(body) != tc.code {
				t.Errorf("= %d %v, want %d %s", status, body, tc.status, tc.code)
			}
		})
	}
}

func TestConsentAndForgetAnswers(t *testing.T) {
	now := clock.NewManual(t0)
	at := t0
	shown := t0.Add(-time.Minute)
	fake := &fakeStore{link: links.Link{Status: links.StatusConsented, NoticeShownAt: &shown, ConsentAt: &at, AgeConfirmedAt: &at},
		forget: links.ForgetResult{Deleted: true, PlayerIDDetached: ptr("player-A")}}
	h := &handlers.Links{Store: fake, Clock: now}

	status, body := call(t, h.Consent, account(`,"notice_shown":true,"consent":true,"age_confirmed":true,"shown_at":"2026-09-13T09:59:00Z"`))
	if status != 200 || body["link_status"] != links.StatusConsented || body["notice_shown_at"] != "2026-09-13T09:59:00Z" {
		t.Errorf("Consent = %d %v", status, body)
	}
	if !fake.gotForm.Complete() || !fake.gotForm.ShownAt.Equal(shown) {
		t.Errorf("the store got the form %+v", fake.gotForm)
	}
	status, body = call(t, h.Forget, account(""))
	if status != 200 || body["deleted"] != true || body["player_id_detached"] != "player-A" {
		t.Errorf("Forget = %d %v", status, body)
	}
	fake.forget = links.ForgetResult{}
	status, body = call(t, h.Forget, account(""))
	if v, has := body["player_id_detached"]; status != 200 || body["deleted"] != false || !has || v != nil {
		t.Errorf("Forget without a link = %d %v, want deleted false and player_id_detached null", status, body)
	}
}

// 503 forget_incomplete tells the client when to repeat; the other answers of
// /forget carry no Retry-After.
func TestForgetIncompleteCarriesRetryAfter(t *testing.T) {
	h := &handlers.Links{Store: &fakeStore{forget: links.ForgetResult{Deleted: true},
		err: fmt.Errorf("%w: busy", links.ErrCompactionPending)}, Clock: clock.NewManual(t0)}
	rec := httptest.NewRecorder()
	h.Forget(rec, httptest.NewRequest(http.MethodDelete, "/", strings.NewReader(account(""))))
	if rec.Code != 503 || rec.Header().Get(api.HeaderRetryAfter) != "5" {
		t.Errorf("= %d Retry-After %q, want 503 and 5", rec.Code, rec.Header().Get(api.HeaderRetryAfter))
	}

	h.Store = &fakeStore{forget: links.ForgetResult{Deleted: true}}
	rec = httptest.NewRecorder()
	h.Forget(rec, httptest.NewRequest(http.MethodDelete, "/", strings.NewReader(account(""))))
	if rec.Code != 200 || rec.Header().Get(api.HeaderRetryAfter) != "" {
		t.Errorf("= %d Retry-After %q, want 200 without it", rec.Code, rec.Header().Get(api.HeaderRetryAfter))
	}
}

// A consented row without one of its timestamps breaks the invariant of the
// store: the answer is 500 internal, not a panic.
func TestConsentOfALinkWithoutItsTimestampsIsInternal(t *testing.T) {
	at := t0
	for name, l := range map[string]links.Link{
		"no consent_at":       {Status: links.StatusConsented, NoticeShownAt: &at, AgeConfirmedAt: &at},
		"no age_confirmed_at": {Status: links.StatusConsented, NoticeShownAt: &at, ConsentAt: &at},
		"no notice_shown_at":  {Status: links.StatusConsented, ConsentAt: &at, AgeConfirmedAt: &at},
	} {
		t.Run(name, func(t *testing.T) {
			h := &handlers.Links{Store: &fakeStore{link: l}, Clock: clock.NewManual(t0)}
			status, body := call(t, h.Consent, account(`,"notice_shown":true,"consent":true,"age_confirmed":true,"shown_at":"2026-09-13T09:59:00Z"`))
			if status != 500 || code(body) != api.CodeInternal {
				t.Errorf("= %d %v, want 500 internal", status, body)
			}
		})
	}
}
