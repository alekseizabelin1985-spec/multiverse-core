package gateway

import (
	"context"
	"errors"
	"strconv"
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
