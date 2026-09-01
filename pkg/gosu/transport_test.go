package gosu

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// scriptedTransport returns a preset status code per call, recording how many calls it
// saw, so a test can assert the transport's retry behavior on 429.
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

// TestTransportRetriesOn429 verifies a 429 is waited out and the request retried until it
// succeeds.
func TestTransportRetriesOn429(t *testing.T) {
	rt := &scriptedTransport{codes: []int{429, 429, 200}, retryAfter: "0"}
	tr := &transport{base: rt}

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
	tr := &transport{base: rt}

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

// TestTransportStampsToken verifies the token is fetched and stamped on the outgoing
// request, alongside the api version.
func TestTransportStampsToken(t *testing.T) {
	rt := &roundTripFunc{body: "{}"}
	tok := &GuestToken{TokenType: "Bearer", AccessToken: "abc"}
	tr := &transport{token: func() (Token, error) { return tok, nil }, base: rt}

	req, _ := http.NewRequest(http.MethodGet, "http://example/x", nil)
	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if got := rt.lastHeader.Get(authHeader); got != "Bearer abc" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer abc")
	}
	if got := rt.lastHeader.Get(apiVersionHeader); got != APIVersion {
		t.Errorf("x-api-version = %q, want %q", got, APIVersion)
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
