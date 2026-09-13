package links

import (
	"context"
	"fmt"
)

// ForgetHooks is one step of the /forget cascade that runs before the link is
// deleted: while the link exists, a failed step can be repeated by repeating
// /forget. The order of C-04 v1.3 and component §7.5 is the order the hooks
// are given to NewSQLite: the outbox is dropped, the character is proposed
// abandoned and leaves its group, the session ends with end_reason=forget.
// The hooks are registered by the packages that own those steps (outbox,
// session, groups); a hook must be idempotent, because a repeat after a
// failure runs every hook again.
type ForgetHooks interface {
	OnForget(ctx context.Context, playerID string) error
}

// ForgetFunc adapts a function to ForgetHooks.
type ForgetFunc func(ctx context.Context, playerID string) error

// OnForget calls f.
func (f ForgetFunc) OnForget(ctx context.Context, playerID string) error { return f(ctx, playerID) }

// ForgetResult is what one call of /forget removed. Deleted is true when this
// call deleted a link; PlayerIDDetached is the character the link pointed at,
// nil when there was no link or no character.
type ForgetResult struct {
	Deleted          bool
	PlayerIDDetached *string
}

// forgetAttempts bounds how often Forget starts the cascade over because the
// character of the link changed while the hooks ran.
const forgetAttempts = 3

// Forget runs the cascade for the link of an external account:
//
//  1. the ForgetHooks, in order, when the link has a character; a failing hook
//     stops the cascade with the link intact;
//  2. DELETE of the link, which cascades to character_requests, on the
//     condition that the link still points at the character the hooks ran
//     for;
//  3. store.CompactLinks, before the answer (ADR-019 addendum p. 1).
//
// When step 2 deletes nothing, the link changed after it was read: a parallel
// /forget deleted it, and this call answers {deleted: false}; or AttachPlayer
// bound another character, and the cascade starts over for that one, at most
// forgetAttempts times in all.
//
// When step 3 fails — a checkpoint blocked by a reader outside the process,
// such as a backup, past busy_timeout, or the deadline of the request — the
// link is already deleted and Forget returns the result together with
// ErrCompactionPending. The caller must not report the data as gone: the HTTP
// layer answers 503 forget_incomplete and the client repeats. While the
// compaction stays pending every Forget returns ErrCompactionPending, whether
// or not its account has a link; the one that finishes the compaction answers
// normally, a repeat of the forgotten account {deleted: false}. The hourly
// Compact, every Sweep and the start of the gateway finish it as well.
//
// Without a link Forget deletes nothing and answers {deleted: false}, after
// finishing a pending compaction. A link without a character is deleted with
// PlayerIDDetached nil: the client says "nothing to delete" for the character
// (US-009), and the external ID is gone all the same.
func (s *SQLite) Forget(ctx context.Context, platform, externalID string) (ForgetResult, error) {
	return s.forget(ctx, func(ctx context.Context) (Link, bool, error) {
		return s.ByExternal(ctx, platform, externalID)
	})
}

func (s *SQLite) ForgetByPlayer(ctx context.Context, playerID string) (ForgetResult, error) {
	return s.forget(ctx, func(ctx context.Context) (Link, bool, error) {
		return s.ByPlayer(ctx, playerID)
	})
}

// forget runs the cascade on the link lookup finds. A repeat after a changed
// link reads it again by link_id: a link created for the account after this
// call began belongs to a new player and is not this call's to delete.
func (s *SQLite) forget(ctx context.Context, lookup func(context.Context) (Link, bool, error)) (ForgetResult, error) {
	for range forgetAttempts {
		link, found, err := lookup(ctx)
		if err != nil {
			return ForgetResult{}, err
		}
		if !found {
			if s.pending.Load() {
				if err := s.Compact(ctx); err != nil {
					return ForgetResult{}, fmt.Errorf("%w: %w", ErrCompactionPending, err)
				}
			}
			return ForgetResult{}, nil
		}
		if link.PlayerID != nil {
			for i, h := range s.hooks {
				if err := h.OnForget(ctx, *link.PlayerID); err != nil {
					return ForgetResult{}, fmt.Errorf("links: forget: hook %d: %w", i, err)
				}
			}
		}
		deleted, err := s.deleteLink(ctx, link)
		if err != nil {
			return ForgetResult{}, err
		}
		if !deleted {
			linkID := link.LinkID
			lookup = func(ctx context.Context) (Link, bool, error) {
				link, found, err := selectLink(ctx, s.db, "link_id = ?", linkID)
				if err != nil {
					return Link{}, false, fmt.Errorf("links: forget: read the link again: %w", err)
				}
				return link, found, nil
			}
			continue
		}
		res := ForgetResult{Deleted: true, PlayerIDDetached: link.PlayerID}
		if err := s.Compact(ctx); err != nil {
			return res, fmt.Errorf("%w: %w", ErrCompactionPending, err)
		}
		return res, nil
	}
	return ForgetResult{}, fmt.Errorf("links: forget: the character of the link changed %d times during the cascade", forgetAttempts)
}

// deleteLink deletes link if it still points at the character read with it
// and reports whether it did. "IS" compares NULL with NULL as equal.
func (s *SQLite) deleteLink(ctx context.Context, link Link) (bool, error) {
	var playerID any
	if link.PlayerID != nil {
		playerID = *link.PlayerID
	}
	r, err := s.db.ExecContext(ctx, "DELETE FROM links WHERE link_id = ? AND player_id IS ?", link.LinkID, playerID)
	if err != nil {
		return false, fmt.Errorf("links: forget: delete: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("links: forget: delete: %w", err)
	}
	return n > 0, nil
}
