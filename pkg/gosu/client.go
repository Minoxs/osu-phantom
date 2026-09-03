package gosu

import (
	"net/http"
	"time"
)

// Client calls the osu! API v2. Auth and pacing live in its transport, so its methods only
// build and decode; a Client is only as paced as the limiter behind it.
type Client struct {
	http  *http.Client
	token func() (Token, error)
}

func newClient(l *RateLimiter, prio Priority, token func() (Token, error), opts []Option) *Client {
	cfg := buildConfig(opts)
	tr := &transport{limiter: l, prio: prio, token: token, base: cfg.base}
	return &Client{http: &http.Client{Timeout: 30 * time.Second, Transport: tr}, token: token}
}

// Validate acquires the token now, so a service can fail on boot against bad credentials
// rather than on its first request.
func (c *Client) Validate() error {
	_, err := c.token()
	return err
}

// NewClient builds a Client on its own limiter paced to the osu! ceiling, fed by a guest
// token it acquires lazily on the first request. osu! counts requests per OAuth client, so
// to hold several clients under one ceiling build an App instead.
func NewClient(creds Credentials, opts ...Option) *Client {
	cfg := buildConfig(opts)
	src := NewGuestTokenProvider(NewOAuth(creds, opts...))
	return NewClientWith(NewRateLimiter(cfg.rate), 0, src, opts...)
}

// NewClientWith builds a Client that reserves against the shared limiter l at priority prio
// and stamps the token src yields.
func NewClientWith(l *RateLimiter, prio Priority, src TokenSource, opts ...Option) *Client {
	return newClient(l, prio, src.Token, opts)
}

// ResourceClient is a Client bound to a resource-owner token. It embeds *Client for every
// public endpoint and adds the user-scoped ones. Building it requires a ResourceOwnerSource,
// so a user-scoped call is unreachable with a guest token at compile time.
type ResourceClient struct {
	*Client
}

// NewResourceClient builds a ResourceClient reserving against l at priority prio, stamping
// the token src yields.
func NewResourceClient(l *RateLimiter, prio Priority, src ResourceOwnerSource, opts ...Option) *ResourceClient {
	token := func() (Token, error) { return src.ResourceToken() }
	return &ResourceClient{Client: newClient(l, prio, token, opts)}
}

// GetOwnUser fetches the osu!standard profile of the user the resource token belongs to.
func (c *ResourceClient) GetOwnUser() (*UserExtended, error) {
	req, _ := http.NewRequest(http.MethodGet, APIv2URL("me/osu"), nil)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	return decodeUserExtended(res)
}
