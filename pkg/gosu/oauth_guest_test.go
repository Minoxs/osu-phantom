package gosu

import (
	"strings"
	"testing"
	"time"
)

func TestGuestTokenAuthorization(t *testing.T) {
	tok := &GuestToken{TokenType: "Bearer", AccessToken: "abc"}
	if got := tok.Authorization(); got != "Bearer abc" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer abc")
	}
}

func TestGuestTokenExpiry(t *testing.T) {
	now := time.Now()
	tok := &GuestToken{ExpiresIn: 3600, ObtainedAt: now}
	if want := now.Add(time.Hour); !tok.ExpiresAt().Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", tok.ExpiresAt(), want)
	}
	if tok.Expired() {
		t.Error("fresh token reports expired")
	}
	tok.ObtainedAt = now.Add(-2 * time.Hour)
	if !tok.Expired() {
		t.Error("stale token reports not expired")
	}
}

func TestOAuthGuestStampsObtainedAt(t *testing.T) {
	rt := &roundTripFunc{body: `{"token_type":"Bearer","expires_in":86400,"access_token":"at"}`}
	tok, err := oauthWith(rt).Guest()
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "at" || tok.ExpiresIn != 86400 {
		t.Errorf("token = %+v, want access at, expires 86400", tok)
	}
	if tok.ObtainedAt.IsZero() {
		t.Error("ObtainedAt not stamped")
	}
	if !strings.HasSuffix(rt.lastURL, "/oauth/token") {
		t.Errorf("posted to %s, want /oauth/token", rt.lastURL)
	}
}

// TestGuestTokenProviderCaches verifies a fresh cached token is reused rather than refetched.
func TestGuestTokenProviderCaches(t *testing.T) {
	rt := &roundTripFunc{body: `{"token_type":"Bearer","expires_in":86400,"access_token":"at"}`}
	p := NewGuestTokenProvider(Credentials{ClientID: 1, ClientSecret: "s"}, WithTransport(rt))

	if _, err := p.Token(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Token(); err != nil {
		t.Fatal(err)
	}
	if rt.calls != 1 {
		t.Errorf("grant calls = %d, want 1 (second Token cached)", rt.calls)
	}
}

// TestClientValidateSurfacesBadCreds verifies Client.Validate returns the grant error at boot.
func TestClientValidateSurfacesBadCreds(t *testing.T) {
	rt := &roundTripFunc{body: `{"error":"invalid_client"}`, status: 401}
	src := NewGuestTokenProvider(Credentials{ClientID: 1, ClientSecret: "bad"}, WithTransport(rt))
	c := NewClientWith(NewRateLimiter(60), 0, src)

	if err := c.Validate(); err == nil {
		t.Error("Validate with rejected creds returned nil, want error")
	}
}

func TestStaticTokenProviderSetToken(t *testing.T) {
	p := NewStaticTokenProvider(&GuestToken{TokenType: "Bearer", AccessToken: "first"})
	got, _ := p.Token()
	if got.Authorization() != "Bearer first" {
		t.Errorf("first token = %q", got.Authorization())
	}
	p.SetToken(&GuestToken{TokenType: "Bearer", AccessToken: "second"})
	got, _ = p.Token()
	if got.Authorization() != "Bearer second" {
		t.Errorf("after SetToken = %q, want Bearer second", got.Authorization())
	}
}
