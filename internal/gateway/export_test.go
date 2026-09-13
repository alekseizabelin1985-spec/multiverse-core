package gateway

import (
	"context"
	"errors"
	"strconv"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/objstore"
)

// SetLinksBusyTimeout sets busy_timeout on the one connection of links.db of a
// started context, so that a test with a reader blocking the checkpoint does
// not wait the five seconds of the store for every compaction.
func SetLinksBusyTimeout(c *Context, millis int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return errors.New("gateway: not started")
	}
	_, err := c.linksDB.ExecContext(context.Background(), "PRAGMA busy_timeout = "+strconv.Itoa(millis))
	return err
}

// DatabasesClosed reports whether both databases of a context that was started
// once are closed: a ping of a closed *sql.DB fails without touching a file.
func DatabasesClosed(c *Context) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx := context.Background()
	return c.linksDB.PingContext(ctx) != nil && c.gatewayDB.PingContext(ctx) != nil
}

// SetObjectStore gives a context that is not started yet the object store its
// projection is loaded from, in place of a client over MV_MINIO_*.
func SetObjectStore(c *Context, objects objstore.Client) { c.objects = objects }

// ReadModel is the projection of a started context.
func ReadModel(c *Context) *readmodel.Model {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.model
}

// Limiter is the rate limit of a started context; nil in replay.
func Limiter(c *Context) *actions.Limiter {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.limiter
}

// PendingActions is the number of half published actions a started context
// holds in memory.
func PendingActions(c *Context) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.service.Pending()
}

// SetStartBudgets shortens the budgets of the load of the snapshot and of the
// catch-up of a context that is not started yet.
func SetStartBudgets(c *Context, load, catchUp time.Duration) {
	c.loadBudget, c.catchUpBudget = load, catchUp
}
