package gosu

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestPacerPrefersHighPriority verifies that a high-priority reservation enqueued
// after a low-priority one is still granted first.
func TestPacerPrefersHighPriority(t *testing.T) {
	l := NewRateLimiter(600) // 100ms between slots
	ctx := context.Background()

	// The first reservation takes the immediate slot and puts the granting loop into
	// its inter-slot sleep, so the next two reservations queue behind one grant.
	if err := l.Reserve(ctx, 0); err != nil {
		t.Fatalf("first reserve: %v", err)
	}

	order := make(chan string, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = l.Reserve(ctx, 0)
		order <- "low"
	}()
	// Let the low reservation reach the queue before the high one, so priority,
	// not arrival order, decides.
	time.Sleep(20 * time.Millisecond)
	go func() {
		defer wg.Done()
		_ = l.Reserve(ctx, 1)
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
	l := NewRateLimiter(600) // 100ms between slots
	ctx := context.Background()

	// The first reservation takes the immediate slot and starts the inter-slot
	// sleep, so the next three queue behind one grant.
	if err := l.Reserve(ctx, 0); err != nil {
		t.Fatalf("first reserve: %v", err)
	}

	order := make(chan int, 3)
	var wg sync.WaitGroup
	reserveAt := func(level Priority) {
		defer wg.Done()
		_ = l.Reserve(ctx, level)
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
	l := NewRateLimiter(1) // one slot per minute, so the second reservation waits

	if err := l.Reserve(context.Background(), 0); err != nil {
		t.Fatalf("first reserve: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Reserve(ctx, 0); err == nil {
		t.Error("reserve with cancelled context returned nil, want context error")
	}
}
