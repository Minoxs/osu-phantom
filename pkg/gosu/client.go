package gosu

import (
	"net/http"
	"time"
)

// Client makes osu! API requests at a fixed pacer priority. Build one per level;
// every request it makes reserves a slot on the shared pacer at that level, so the
// whole process stays under the API ceiling while higher-priority clients are served
// ahead of lower ones. The priority is the client's, not the call's: a caller picks
// the level once when it builds the client, and the request methods carry none of
// their own.
type Client struct {
	http *http.Client
}

// NewClient builds a Client whose every request reserves at prio on the shared pacer.
func NewClient(prio Priority) *Client {
	return &Client{http: &http.Client{
		Timeout:   30 * time.Second,
		Transport: &throttledTransport{base: http.DefaultTransport, pacer: globalPacer, prio: prio},
	}}
}
