package gosu

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestResourceTokenAuthorization(t *testing.T) {
	tok := &ResourceToken{TokenType: "Bearer", AccessToken: "xyz"}
	if got := tok.Authorization(); got != "Bearer xyz" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer xyz")
	}
}

func TestResourceTokenExpiry(t *testing.T) {
	now := time.Now()
	tok := &ResourceToken{ExpiresIn: 3600, ObtainedAt: now}
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

func TestOAuthExchange(t *testing.T) {
	rt := &roundTripFunc{body: `{"token_type":"Bearer","expires_in":86400,"access_token":"at","refresh_token":"rt"}`}
	tok, err := oauthWith(rt).Exchange("code", "https://app/cb")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "at" || tok.RefreshToken != "rt" {
		t.Errorf("token = %+v, want access at, refresh rt", tok)
	}
	if tok.ObtainedAt.IsZero() {
		t.Error("ObtainedAt not stamped")
	}
	if !strings.HasSuffix(rt.lastURL, "/oauth/token") {
		t.Errorf("posted to %s, want /oauth/token", rt.lastURL)
	}
}

func TestOAuthRefresh(t *testing.T) {
	rt := &roundTripFunc{body: `{"token_type":"Bearer","expires_in":86400,"access_token":"new","refresh_token":"rotated"}`}
	tok, err := oauthWith(rt).Refresh("original")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "new" || tok.RefreshToken != "rotated" {
		t.Errorf("token = %+v, want access new, refresh rotated", tok)
	}
}

// TestResourceOwnerTokenProviderRefreshesWhenStale verifies a stale token is refreshed and
// the rotated refresh token stored.
func TestResourceOwnerTokenProviderRefreshesWhenStale(t *testing.T) {
	rt := &roundTripFunc{body: `{"token_type":"Bearer","expires_in":86400,"access_token":"new","refresh_token":"rotated"}`}
	initial := &ResourceToken{AccessToken: "old", RefreshToken: "original", ExpiresIn: 3600, ObtainedAt: time.Now().Add(-2 * time.Hour)}
	p := NewResourceOwnerTokenProvider(oauthWith(rt), initial)

	got, err := p.ResourceToken()
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "new" || got.RefreshToken != "rotated" {
		t.Errorf("refreshed = %+v, want access new, refresh rotated", got)
	}
	if rt.calls != 1 {
		t.Errorf("grant calls = %d, want 1", rt.calls)
	}

	// The refreshed token is fresh, so a second read is cached.
	if _, err := p.ResourceToken(); err != nil {
		t.Fatal(err)
	}
	if rt.calls != 1 {
		t.Errorf("grant calls = %d after cached read, want 1", rt.calls)
	}
}

func TestResourceOwnerTokenProviderNoToken(t *testing.T) {
	rt := &roundTripFunc{}
	p := NewResourceOwnerTokenProvider(oauthWith(rt), nil)
	if _, err := p.ResourceToken(); !errors.Is(err, ErrNoResourceToken) {
		t.Errorf("err = %v, want ErrNoResourceToken", err)
	}
}

func TestStaticResourceOwnerTokenProviderSetToken(t *testing.T) {
	p := NewStaticResourceOwnerTokenProvider(&ResourceToken{AccessToken: "first"})
	got, _ := p.ResourceToken()
	if got.AccessToken != "first" {
		t.Errorf("first token = %q, want first", got.AccessToken)
	}
	p.SetToken(&ResourceToken{AccessToken: "second"})
	got, _ = p.ResourceToken()
	if got.AccessToken != "second" {
		t.Errorf("after SetToken = %q, want second", got.AccessToken)
	}
}
