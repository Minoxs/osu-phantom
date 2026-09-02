package gosu

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

// defaultRequestsPerMinute is the osu! API terms-of-use ceiling.
const defaultRequestsPerMinute = 60

// Priority orders requests competing for one RateLimiter's slots: higher wins, ties
// break by arrival. It matters only when several transports share a limiter.
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

// RateLimiter schedules dispatch slots to keep traffic under the osu! ceiling. It grants
// one slot per interval to the highest-priority waiter. Share one to hold a single ceiling
// across every client and token fetch under an OAuth app. Its granting goroutine runs only
// while work is queued and exits once idle, so an unused limiter parks nothing.
type RateLimiter struct {
	mu       sync.Mutex
	queue    waiterHeap
	seq      uint64
	interval time.Duration
	started  bool
}

// NewRateLimiter builds a limiter pacing to perMinute per minute. A non-positive perMinute
// uses the osu! ceiling.
func NewRateLimiter(perMinute int) *RateLimiter {
	return &RateLimiter{interval: rateInterval(perMinute)}
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

// Reserve blocks until the limiter grants a slot or ctx is cancelled.
func (l *RateLimiter) Reserve(ctx context.Context, prio Priority) error {
	w := &waiter{prio: prio, ready: make(chan struct{})}

	l.mu.Lock()
	if !l.started {
		l.started = true
		go l.run()
	}
	w.seq = l.seq
	l.seq++
	heap.Push(&l.queue, w)
	l.mu.Unlock()

	select {
	case <-w.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run grants one waiter per interval, highest priority first, exiting once the queue drains
// so an idle limiter parks no goroutine. Reserve restarts it.
func (l *RateLimiter) run() {
	for {
		l.mu.Lock()
		if l.queue.Len() == 0 {
			l.started = false
			l.mu.Unlock()
			return
		}
		w := heap.Pop(&l.queue).(*waiter)
		interval := l.interval
		l.mu.Unlock() // before the sleep, or every reserve blocks a full interval

		close(w.ready)
		time.Sleep(interval)
	}
}
