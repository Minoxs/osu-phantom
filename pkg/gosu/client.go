package gosu

import (
	"net/http"
	"time"
)

// Client calls the osu! API v2. Its methods take a GuestToken and return decoded
// results. Pacing lives in the transport, so a Client is only as paced as its limiter.
type Client struct {
	http *http.Client
}

// NewClient builds a Client on its own RateLimiter paced to the osu! ceiling. osu! counts
// requests per OAuth client, so several clients under one app must share a limiter via
// NewClientWith rather than each building their own.
func NewClient() *Client {
	return NewClientWith(NewRateLimiter(http.DefaultTransport, defaultRequestsPerMinute), 0)
}

// NewClientWith builds a Client that submits to the shared limiter l at priority prio.
func NewClientWith(l *RateLimiter, prio Priority) *Client {
	return &Client{http: &http.Client{
		Timeout:   30 * time.Second,
		Transport: prioTransport{l: l, prio: prio},
	}}
}
