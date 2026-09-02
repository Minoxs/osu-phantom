package gosu

import "net/http"

// Option configures a client or the OAuth grant type at construction.
type Option func(*config)

type config struct {
	base    http.RoundTripper
	limiter *RateLimiter
}

func buildConfig(opts []Option) config {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.base == nil {
		cfg.base = http.DefaultTransport
	}
	return cfg
}

// WithTransport sets the network transport under the pacing layer, the point a proxy, logger,
// or custom TLS plugs into. It defaults to http.DefaultTransport.
func WithTransport(base http.RoundTripper) Option {
	return func(c *config) { c.base = base }
}

// WithLimiter paces the OAuth token grants through l. Grants are off the osu! api/v2 ceiling
// and unpaced by default; set this only to fold them under a shared limiter, where they
// reserve at the top priority so a refresh is never starved behind work blocked on it.
func WithLimiter(l *RateLimiter) Option {
	return func(c *config) { c.limiter = l }
}
