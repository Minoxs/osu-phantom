package gosu

import (
	"container/heap"
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// defaultRequestsPerMinute is the osu! API terms-of-use ceiling: no more than 60
// requests per minute. Every request this package makes passes through the pacer
// so the whole process, across all callers, stays under it.
const defaultRequestsPerMinute = 60

// osu! counts requests per OAuth client and its window outlives a process restart, so a
// redeploy can 429 despite the pacer. These bound the wait-and-retry that absorbs it.
const (
	maxRateLimitRetries = 4
	defaultRetryAfter   = 2 * time.Second
	maxRetryAfter       = time.Minute
)

// Priority orders requests at the shared pacer: when several wait at once, a higher
// value is granted first, and ties break by arrival order. This package only orders
// the levels; the caller assigns their meaning by building a Client at each level.
// The zero value is the level a default Client carries.
type Priority int

// waiter is one pending reservation; ready closes when the pacer grants it.
type waiter struct {
	prio  Priority
	seq   uint64
	ready chan struct{}
}

// waiterHeap orders by descending priority, then ascending seq so a level is served
// first-come first-served.
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

// pacer spaces osu! requests to keep the whole process under the API ceiling,
// granting one slot per interval to the highest-priority waiter. A caller reserves
// at a priority it chooses, so a burst of low-priority traffic never delays a
// higher-priority request by more than a single slot.
type pacer struct {
	mu       sync.Mutex
	cond     *sync.Cond
	queue    waiterHeap
	seq      uint64
	interval time.Duration
	started  bool
}

func newPacer(perMinute int) *pacer {
	p := &pacer{interval: rateInterval(perMinute)}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func rateInterval(perMinute int) time.Duration {
	if perMinute < 1 {
		perMinute = 1
	}
	return time.Minute / time.Duration(perMinute)
}

// reserve blocks until the pacer grants a slot or ctx is cancelled. A higher
// priority is granted ahead of any lower one waiting at the same time.
func (p *pacer) reserve(ctx context.Context, prio Priority) error {
	w := &waiter{prio: prio, ready: make(chan struct{})}

	p.mu.Lock()
	if !p.started {
		p.started = true
		go p.run()
	}
	w.seq = p.seq
	p.seq++
	heap.Push(&p.queue, w)
	p.cond.Signal()
	p.mu.Unlock()

	select {
	case <-w.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run grants one waiter per interval, always the highest priority waiting.
func (p *pacer) run() {
	for {
		p.mu.Lock()
		for p.queue.Len() == 0 {
			p.cond.Wait()
		}
		w := heap.Pop(&p.queue).(*waiter)
		interval := p.interval
		p.mu.Unlock()

		close(w.ready)
		time.Sleep(interval)
	}
}

func (p *pacer) setRate(perMinute int) {
	p.mu.Lock()
	p.interval = rateInterval(perMinute)
	p.mu.Unlock()
}

// throttledTransport blocks each request until the pacer grants it a slot at prio,
// the level of the Client that owns this transport. The request context is honored
// only for cancellation.
type throttledTransport struct {
	base  http.RoundTripper
	pacer *pacer
	prio  Priority
}

// RoundTrip paces each attempt and, on a 429, waits out the limit and retries up to
// maxRateLimitRetries times. Each attempt reserves its own pacer slot so retries do not
// burst. An unrewindable body or a persistent 429 is returned as-is.
func (t *throttledTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		if err := t.pacer.reserve(req.Context(), t.prio); err != nil {
			return nil, err
		}
		res, err := t.base.RoundTrip(req)
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

// rewind resets a request body so it can be sent again, reporting whether the request is
// safe to retry. A bodyless request always is; one with a body needs GetBody.
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

// retryAfter reads the Retry-After header as delta seconds or an HTTP date, falling back
// to def when absent or unparseable and capping at maxRetryAfter.
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

// globalPacer is the process-wide pacer every Client reserves on, so the rate ceiling
// holds across all callers and priority levels.
var globalPacer = newPacer(defaultRequestsPerMinute)

// SetRateLimit caps every osu! request this package makes to perMinute requests
// per minute, shared process-wide. Values below 1 clamp to 1.
func SetRateLimit(perMinute int) {
	globalPacer.setRate(perMinute)
}
