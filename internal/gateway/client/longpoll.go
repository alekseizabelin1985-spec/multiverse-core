package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"multiverse-core.io/internal/gateway/api"
)

// Limits of the long-poll (C-08 v1.1): the gateway rejects larger values, so
// the client clamps them instead of sending a request that cannot succeed.
const (
	MaxPollWait  = 25 * time.Second
	MaxPollLimit = 100
)

// Deliveries takes the next deliveries of this client (GET
// /v1/clients/{client_id}/deliveries). after is the cursor of the previous
// answer, empty on the first call; limit <= 0 and wait < 0 leave the gateway
// defaults.
//
// The long-poll is not repeated inside the client: the gateway allows one
// active poll per client, so a repeat after a network error could meet the
// abandoned poll and answer 409 poll_in_progress. The delivery loop of the
// caller polls again anyway.
func (c *Client) Deliveries(ctx context.Context, after string, limit int, wait time.Duration) (api.DeliveriesResponse, error) {
	q := url.Values{}
	if after != "" {
		q.Set("after", after)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(min(limit, MaxPollLimit)))
	}
	if wait >= 0 {
		q.Set("wait_ms", strconv.FormatInt(min(wait, MaxPollWait).Milliseconds(), 10))
	}
	var out api.DeliveriesResponse
	_, _, err := c.call(ctx, request{method: http.MethodGet, path: c.clientPath("/deliveries"), query: q},
		decodeOn(&out, http.StatusOK))
	return out, err
}

// Ack confirms deliveries by id (POST /v1/clients/{client_id}/deliveries/ack).
// Unknown ids are not an error: they come back in AckResponse.Unknown. Ack is
// repeated after a network error or 503; when the lost attempt had already
// confirmed some ids, the repeat reports them as unknown, so Unknown after a
// failed round trip does not prove a delivery was not confirmed.
func (c *Client) Ack(ctx context.Context, ids []string) (api.AckResponse, error) {
	var out api.AckResponse
	_, _, err := c.call(ctx, request{method: http.MethodPost, path: c.clientPath("/deliveries/ack"), body: api.AckRequest{IDs: ids}, retry: true},
		decodeOn(&out, http.StatusOK))
	return out, err
}

// clientPath builds a path under /v1/clients/{client_id}; the id is the one
// sent in X-Client-Id, so the pair cannot disagree (SEC-12).
func (c *Client) clientPath(suffix string) string {
	return "/v1/clients/" + url.PathEscape(c.ClientID) + suffix
}
