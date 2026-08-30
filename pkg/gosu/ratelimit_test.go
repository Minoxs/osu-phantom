package gosu

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// scriptedTransport returns a preset status code per call, recording how many calls it
// saw, so a test can assert the throttledTransport's retry behavior on 429.
type scriptedTransport struct {
	codes      []int
	retryAfter string
	calls      int
}

func (s *scriptedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	code := s.codes[len(s.codes)-1]
	if s.calls < len(s.codes) {
		code = s.codes[s.calls]
	}
	s.calls++
	h := http.Header{}
	if code == http.StatusTooManyRequests && s.retryAfter != "" {
		h.Set("Retry-After", s.retryAfter)
	}
	return &http.Response{StatusCode: code, Header: h, Body: io.NopCloser(strings.NewReader(""))}, nil
}

// TestTransportRetriesOn429 verifies a 429 is waited out and the request retried until
// it succeeds, each attempt pacing through the shared pacer.
func TestTransportRetriesOn429(t *testing.T) {
	rt := &scriptedTransport{codes: []int{429, 429, 200}, retryAfter: "0"}
	tr := &throttledTransport{base: rt, pacer: newPacer(600), prio: 0}

	req, _ := http.NewRequest(http.MethodGet, "http://example/x", nil)
	res, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
	if rt.calls != 3 {
		t.Errorf("calls = %d, want 3", rt.calls)
	}
}

// TestTransportSurfacesPersistent429 verifies a limit that never clears fails after the
// retry budget rather than looping forever.
func TestTransportSurfacesPersistent429(t *testing.T) {
	rt := &scriptedTransport{codes: []int{429}, retryAfter: "0"}
	tr := &throttledTransport{base: rt, pacer: newPacer(600), prio: 0}

	req, _ := http.NewRequest(http.MethodGet, "http://example/x", nil)
	res, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if res.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", res.StatusCode)
	}
	if rt.calls != maxRateLimitRetries+1 {
		t.Errorf("calls = %d, want %d", rt.calls, maxRateLimitRetries+1)
	}
}

// TestRetryAfterParsing covers the header forms and the fallback.
func TestRetryAfterParsing(t *testing.T) {
	def := 2 * time.Second
	if got := retryAfter(http.Header{}, def); got != def {
		t.Errorf("missing header = %v, want %v", got, def)
	}
	h := http.Header{}
	h.Set("Retry-After", "5")
	if got := retryAfter(h, def); got != 5*time.Second {
		t.Errorf("seconds header = %v, want 5s", got)
	}
	h.Set("Retry-After", "999999")
	if got := retryAfter(h, def); got != maxRetryAfter {
		t.Errorf("oversized header = %v, want cap %v", got, maxRetryAfter)
	}
}

// TestPacerPrefersHighPriority verifies that a high-priority reservation enqueued
// after a low-priority one is still granted first.
func TestPacerPrefersHighPriority(t *testing.T) {
	p := newPacer(600) // 100ms between slots
	ctx := context.Background()

	// The first reservation takes the immediate slot and puts the dispatcher into
	// its inter-slot sleep, so the next two reservations queue behind one grant.
	if err := p.reserve(ctx, 0); err != nil {
		t.Fatalf("first reserve: %v", err)
	}

	order := make(chan string, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = p.reserve(ctx, 0)
		order <- "low"
	}()
	// Let the low reservation reach the queue before the high one, so priority,
	// not arrival order, decides.
	time.Sleep(20 * time.Millisecond)
	go func() {
		defer wg.Done()
		_ = p.reserve(ctx, 1)
		order <- "high"
	}()

	wg.Wait()
	if first := <-order; first != "high" {
		t.Errorf("first granted = %q, want high", first)
	}
}

// TestPacerServesLevelsInDescendingOrder verifies the pacer grants across more than
// two levels strictly highest first, regardless of arrival order.
func TestPacerServesLevelsInDescendingOrder(t *testing.T) {
	p := newPacer(600) // 100ms between slots
	ctx := context.Background()

	// The first reservation takes the immediate slot and starts the inter-slot
	// sleep, so the next three queue behind one grant.
	if err := p.reserve(ctx, 0); err != nil {
		t.Fatalf("first reserve: %v", err)
	}

	order := make(chan int, 3)
	var wg sync.WaitGroup
	reserveAt := func(level Priority) {
		defer wg.Done()
		_ = p.reserve(ctx, level)
		order <- int(level)
	}
	wg.Add(3)
	// Enqueue lowest first, then middle, then highest, so priority not arrival order
	// decides. Small gaps keep the enqueue order deterministic.
	go reserveAt(0)
	time.Sleep(20 * time.Millisecond)
	go reserveAt(1)
	time.Sleep(20 * time.Millisecond)
	go reserveAt(2)

	wg.Wait()
	got := []int{<-order, <-order, <-order}
	want := []int{2, 1, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("grant order = %v, want %v", got, want)
		}
	}
}

// TestPacerReserveHonorsContext verifies a cancelled context releases a waiter
// instead of blocking on a slot that never comes.
func TestPacerReserveHonorsContext(t *testing.T) {
	p := newPacer(1) // one slot per minute, so the second reservation waits

	if err := p.reserve(context.Background(), 0); err != nil {
		t.Fatalf("first reserve: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.reserve(ctx, 0); err == nil {
		t.Error("reserve with cancelled context returned nil, want context error")
	}
}
