package flow

import (
	"time"

	"multiverse-core.io/internal/gateway/api"
)

// DialogTTL is how long a step of the onboarding waits for the player: a
// dialog untouched for this long is gone, and the next command starts from
// links/resolve (component §10.3).
const DialogTTL = 15 * time.Minute

// PlayerTTL is how long the bot trusts the player_id of a chat without asking
// the gateway again (component §10.3).
const PlayerTTL = time.Hour

// Step is a step of the onboarding (component §10.1). A chat without a dialog
// is idle; a chat with a character is ready, which the player cache records,
// not a dialog.
type Step int

// The steps of the onboarding.
const (
	Idle Step = iota
	AwaitingConsent
	AwaitingName
	AwaitingNameConfirm
	AwaitingWorld
	Ready
)

var stepNames = map[Step]string{
	Idle:                "idle",
	AwaitingConsent:     "awaiting_consent",
	AwaitingName:        "awaiting_name",
	AwaitingNameConfirm: "awaiting_name_confirm",
	AwaitingWorld:       "awaiting_world",
	Ready:               "ready",
}

func (s Step) String() string { return stepNames[s] }

// dialog is the onboarding of one chat. It holds what the player typed for
// the character to be created — the name, which is the player's own choice
// (FR-060) — and never anything of the Telegram profile.
type dialog struct {
	step    Step
	touched time.Time
	// noticeShownAt is when the notice was last sent to the chat; it is the
	// shown_at of links/consent.
	noticeShownAt time.Time
	name          string
	worlds        []api.WorldSummary
}

type player struct {
	id      string
	worldID string
	at      time.Time
}

// sweepLocked lets go of the dialogs, players and pending wipes whose time is over,
// so that a chat id stays in memory no longer than its TTL and the next update.
// f.mu must be held.
func (f *Flow) sweepLocked(now time.Time) {
	for chat, d := range f.dialogs {
		if now.Sub(d.touched) >= DialogTTL {
			delete(f.dialogs, chat)
		}
	}
	for chat, p := range f.players {
		if now.Sub(p.at) >= PlayerTTL {
			delete(f.players, chat)
		}
	}
	for chat, at := range f.forgetting {
		if now.Sub(at) >= PlayerTTL {
			delete(f.forgetting, chat)
		}
	}
}

// dialogOf returns a copy of the live dialog of a chat, false when there is
// none. The turn changes its copy and stores it back with setDialog, so that
// StepOf from another goroutine never reads a dialog being changed (review #1
// of T-311, Mi-7).
func (f *Flow) dialogOf(chat int64, now time.Time) (dialog, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.dialogs[chat]
	if ok && now.Sub(d.touched) >= DialogTTL {
		delete(f.dialogs, chat)
		return dialog{}, false
	}
	return d, ok
}

// setDialog stores a copy of the dialog of a chat as touched at now.
func (f *Flow) setDialog(chat int64, d *dialog, now time.Time) {
	stored := *d
	stored.touched = now
	f.mu.Lock()
	f.dialogs[chat] = stored
	f.mu.Unlock()
}

func (f *Flow) dropDialog(chat int64) {
	f.mu.Lock()
	delete(f.dialogs, chat)
	f.mu.Unlock()
}

// playerOf returns the cached player of a chat.
func (f *Flow) playerOf(chat int64, now time.Time) (player, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.players[chat]
	if ok && now.Sub(p.at) >= PlayerTTL {
		delete(f.players, chat)
		return player{}, false
	}
	return p, ok
}

func (f *Flow) setPlayer(chat int64, id, worldID string, now time.Time) {
	f.mu.Lock()
	f.players[chat] = player{id: id, worldID: worldID, at: now}
	f.mu.Unlock()
}

func (f *Flow) dropPlayer(chat int64) {
	f.mu.Lock()
	delete(f.players, chat)
	f.mu.Unlock()
}

// markForgetting remembers that the last /forget of a chat answered
// forget_incomplete, so that {deleted: false} of its repeat reads as the end
// of the same /forget (C-08 v1.5).
func (f *Flow) markForgetting(chat int64, now time.Time) {
	f.mu.Lock()
	f.forgetting[chat] = now
	f.mu.Unlock()
}

func (f *Flow) takeForgetting(chat int64, now time.Time) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	at, ok := f.forgetting[chat]
	delete(f.forgetting, chat)
	return ok && now.Sub(at) < PlayerTTL
}

// Reset forgets everything the flow holds about a chat: the dialog, the
// cached player and a pending wipe. The flow calls it after /forget; the next
// command of the chat starts from links/resolve.
func (f *Flow) Reset(chat int64) {
	f.mu.Lock()
	delete(f.dialogs, chat)
	delete(f.players, chat)
	delete(f.forgetting, chat)
	f.mu.Unlock()
}

// StepOf reports the step of a chat at now: the step of its dialog, Ready
// when a player is cached, Idle otherwise. It is safe to call from another
// goroutine while Handle runs: it reads a copy taken under the mutex.
func (f *Flow) StepOf(chat int64) Step {
	now := f.clock.Now()
	if d, ok := f.dialogOf(chat, now); ok {
		return d.step
	}
	if _, ok := f.playerOf(chat, now); ok {
		return Ready
	}
	return Idle
}
