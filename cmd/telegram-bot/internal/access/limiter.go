package access

import (
	"sync"
	"time"
)

// Window is the span the command limit counts over.
const Window = time.Minute

// limiter admits at most limit commands of one user within any Window
// (NFR-049: "≤ 20 команд/мин на пользователя").
//
// It is a sliding log rather than the token bucket component §10.2 names: a
// bucket of 20 refilled at 20 per minute lets 39 commands through in the first
// minute and admits the 21st three seconds after a burst, while US-008 wants
// the 21st command within a minute refused until the 61st second. The log holds
// at most limit timestamps per user, and only allow-listed users reach it.
type limiter struct {
	limit int
	mu    sync.Mutex
	users map[int64]*userLog
}

type userLog struct {
	hits []time.Time
	// warned is set once the user was told to wait, and cleared by the next
	// admitted command: a flood is answered once, not once per message.
	warned bool
}

func newLimiter(limit int) *limiter {
	return &limiter{limit: limit, users: make(map[int64]*userLog)}
}

// take records a command of user at now. It returns whether the command is
// admitted; when it is not, wait is how long until one is, and notify says
// whether this is the first refusal since the last admitted command.
func (l *limiter) take(user int64, now time.Time) (ok bool, wait time.Duration, notify bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	u := l.users[user]
	if u == nil {
		u = &userLog{}
		l.users[user] = u
	}
	cutoff := now.Add(-Window)
	kept := u.hits[:0]
	for _, hit := range u.hits {
		if hit.After(cutoff) {
			kept = append(kept, hit)
		}
	}
	u.hits = kept
	if len(u.hits) < l.limit {
		u.hits = append(u.hits, now)
		u.warned = false
		return true, 0, false
	}
	wait = u.hits[0].Add(Window).Sub(now)
	notify = !u.warned
	u.warned = true
	return false, wait, notify
}
