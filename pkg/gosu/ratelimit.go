package gosu

import (
	"container/heap"
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// defaultRequestsPerMinute is the osu! API terms-of-use ceiling.
const defaultRequestsPerMinute = 60

// osu! counts requests per OAuth client and the window outlives a restart, so a redeploy
// can 429 despite pacing. These bound the retry that absorbs it.
const (
	maxRateLimitRetries = 4
	defaultRetryAfter   = 2 * time.Second
	maxRetryAfter       = time.Minute
)

// Priority orders requests competing for one RateLimiter's slots: higher wins, ties
// break by arrival. It matters only when several clients share a limiter.
type Priority int

type waiter struct {
	prio  Priority
	seq   uint64
	ready chan struct{}
}

// waiterHeap orders by descending priority, then ascending seq.
type waiterHeap []*waiter

func (h waiterHeap) Len() int { return len(h) }
func (h waiterHeap) Less(i, j int) bool {
	if h[i].prio != h[j].prio {
		return h[i].prio > h[j].prio
	}
	return h[i].seq < h[j].seq
}
func (h waiterHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *waiterHeap) Push(x any)   { *h = append(*h, x.(*waiter)) }
func (h *waiterHeap) Pop() any {
	old := *h
	n := len(old)
	w := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return w
}

// RateLimiter is an http.RoundTripper that spaces requests to keep traffic through it
// under the osu! ceiling and retries on 429. It grants one slot per interval to the
// highest-priority waiter. Share one across clients to hold a single ceiling for them.
type RateLimiter struct {
	base http.RoundTripper

	mu       sync.Mutex
	cond     *sync.Cond
	queue    waiterHeap
	seq      uint64
	interval time.Duration
	started  bool
}

// NewRateLimiter wraps base to pace requests to perMinute per minute. A nil base uses
// http.DefaultTransport; a non-positive perMinute uses the osu! ceiling.
func NewRateLimiter(base http.RoundTripper, perMinute int) *RateLimiter {
	if base == nil {
		base = http.DefaultTransport
	}
	l := &RateLimiter{base: base, interval: rateInterval(perMinute)}
	l.cond = sync.NewCond(&l.mu)
	return l
}

func rateInterval(perMinute int) time.Duration {
	if perMinute < 1 {
		perMinute = defaultRequestsPerMinute
	}
	return time.Minute / time.Duration(perMinute)
}

// SetRate repaces to perMinute per minute from the next slot. Non-positive uses the ceiling.
func (l *RateLimiter) SetRate(perMinute int) {
	l.mu.Lock()
	l.interval = rateInterval(perMinute)
	l.mu.Unlock()
}

// reserve blocks until the limiter grants a slot or ctx is cancelled.
func (l *RateLimiter) reserve(ctx context.Context, prio Priority) error {
	w := &waiter{prio: prio, ready: make(chan struct{})}

	l.mu.Lock()
	if !l.started {
		l.started = true
		go l.run()
	}
	w.seq = l.seq
	l.seq++
	heap.Push(&l.queue, w)
	l.cond.Signal()
	l.mu.Unlock()

	select {
	case <-w.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run grants one waiter per interval, highest priority first.
func (l *RateLimiter) run() {
	for {
		l.mu.Lock()
		for l.queue.Len() == 0 {
			l.cond.Wait()
		}
		w := heap.Pop(&l.queue).(*waiter)
		interval := l.interval
		l.mu.Unlock()

		close(w.ready)
		time.Sleep(interval)
	}
}

// RoundTrip paces each attempt and retries on 429 up to maxRateLimitRetries times, each
// attempt reserving its own slot. An unrewindable body or a persistent 429 returns as-is.
func (l *RateLimiter) RoundTrip(req *http.Request) (*http.Response, error) {
	prio := priorityFrom(req.Context())
	for attempt := 0; ; attempt++ {
		if err := l.reserve(req.Context(), prio); err != nil {
			return nil, err
		}
		res, err := l.base.RoundTrip(req)
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

type priorityKey struct{}

func withPriority(ctx context.Context, prio Priority) context.Context {
	return context.WithValue(ctx, priorityKey{}, prio)
}

func priorityFrom(ctx context.Context) Priority {
	if prio, ok := ctx.Value(priorityKey{}).(Priority); ok {
		return prio
	}
	return 0
}

// prioTransport stamps a Client's priority onto each request so the shared limiter it
// wraps can order requests across clients.
type prioTransport struct {
	l    *RateLimiter
	prio Priority
}

func (t prioTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.l.RoundTrip(req.WithContext(withPriority(req.Context(), t.prio)))
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
