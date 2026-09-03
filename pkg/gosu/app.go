package gosu

// App is one osu! OAuth app: its credentials, the single api/v2 limiter osu! meters the app
// against, and the grant machine. Every client it returns shares that one ceiling, so guest
// reads and per-user resource calls under the app stay under one limit no matter how many
// clients run.
type App struct {
	oauth   *OAuth
	guest   TokenSource
	limiter *RateLimiter
	opts    []Option
}

// NewApp builds an app whose clients share one ceiling and one guest token. RateLimit sets
// that ceiling, Transport the network transport beneath it.
func NewApp(clientID int, clientSecret string, opts ...Option) *App {
	cfg := buildConfig(opts)
	o := NewOAuth(Credentials{ClientID: clientID, ClientSecret: clientSecret}, opts...)
	return &App{
		oauth:   o,
		guest:   NewGuestTokenProvider(o),
		limiter: NewRateLimiter(cfg.rate),
		opts:    opts,
	}
}

// Validate acquires the guest token now, so a caller can fail on boot against bad credentials
// rather than on the first request.
func (a *App) Validate() error {
	_, err := a.guest.Token()
	return err
}

// GuestClient returns a client on the app-wide guest token, reserving against the shared ceiling
// at prio. Priority orders requests only when several clients share the app.
func (a *App) GuestClient(prio Priority) *Client {
	return NewClientWith(a.limiter, prio, a.guest, a.opts...)
}

// ResourceOwnerClient returns a client bound to one user's token, reserving against the same
// shared ceiling at prio. The token refreshes itself, storing the refresh token osu! rotates.
func (a *App) ResourceOwnerClient(prio Priority, tok *ResourceToken) *ResourceClient {
	src := NewResourceOwnerTokenProvider(a.oauth, tok)
	return NewResourceClient(a.limiter, prio, src, a.opts...)
}

// OAuth is the shared grant machine, for the authorize, exchange, and refresh flow.
func (a *App) OAuth() *OAuth { return a.oauth }
