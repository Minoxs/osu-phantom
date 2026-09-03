package gosu

import "net/http"

// Option configures a client or the OAuth grant type at construction.
type Option func(*config)

type config struct {
	base http.RoundTripper
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

// Transport sets the network transport under the pacing layer, the point a proxy, logger,
// or custom TLS plugs into. It defaults to http.DefaultTransport.
func Transport(base http.RoundTripper) Option {
	return func(c *config) { c.base = base }
}
