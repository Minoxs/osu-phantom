package gosu

import "net/http"

// Option configures a client or the OAuth grant type at construction.
type Option func(*config)

type config struct {
	base http.RoundTripper
}

func buildConfig(opts []Option) config {
	cfg := config{base: http.DefaultTransport}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.base == nil {
		cfg.base = http.DefaultTransport
	}
	return cfg
}

// WithTransport sets the network transport under the pacing layer, the point a proxy or
// custom TLS plugs into. It defaults to http.DefaultTransport.
func WithTransport(base http.RoundTripper) Option {
	return func(c *config) { c.base = base }
}
