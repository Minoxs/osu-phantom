package gosu

import (
	"net/http"
	"strconv"
	"time"
)

// osu! counts requests per OAuth client and the window outlives a restart, so a redeploy
// can 429 despite pacing. These bound the retry that absorbs it.
const (
	maxRateLimitRetries = 4
	defaultRetryAfter   = 2 * time.Second
	maxRetryAfter       = time.Minute
)

// transport reserves a dispatch slot, then stamps the token, then sends. Reserving before
// stamping is what keeps a queued request from carrying a token that expired while it
// waited: token runs at dispatch, not at enqueue. A nil limiter skips pacing; a nil token
// stamps no credential, which is what the OAuth grant calls use.
type transport struct {
	limiter *RateLimiter
	prio    Priority
	token   func() (Token, error)
	base    http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		if t.limiter != nil {
			if err := t.limiter.Reserve(req.Context(), t.prio); err != nil {
				return nil, err
			}
		}

		out := req
		if t.token != nil {
			tok, err := t.token()
			if err != nil {
				return nil, err
			}
			out = req.Clone(req.Context())
			out.Header.Set(authHeader, tok.Authorization())
			out.Header.Set(apiVersionHeader, APIVersion)
		}

		res, err := t.base.RoundTrip(out)
		if err != nil {
			return nil, err
		}
		if res.StatusCode != http.StatusTooManyRequests || attempt >= maxRateLimitRetries || !rewind(req) {
			return res, nil
		}
		wait := retryAfter(res.Header, defaultRetryAfter)
		res.Body.Close()
		select {
		case <-time.After(wait):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
}

// rewind resets a request body for a retry, reporting whether that is possible. A body
// can only be replayed through GetBody.
func rewind(req *http.Request) bool {
	if req.Body == nil || req.Body == http.NoBody {
		return true
	}
	if req.GetBody == nil {
		return false
	}
	body, err := req.GetBody()
	if err != nil {
		return false
	}
	req.Body = body
	return true
}

// retryAfter reads Retry-After as delta seconds or an HTTP date, falling back to def and
// capping at maxRetryAfter.
func retryAfter(h http.Header, def time.Duration) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return def
	}
	wait := def
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		wait = time.Duration(secs) * time.Second
	} else if when, err := http.ParseTime(v); err == nil {
		if d := time.Until(when); d > 0 {
			wait = d
		}
	}
	if wait > maxRetryAfter {
		return maxRetryAfter
	}
	return wait
}
