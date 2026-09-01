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

func TestGetGuestTokenStampsObtainedAt(t *testing.T) {
	rt := &roundTripFunc{body: `{"token_type":"Bearer","expires_in":86400,"access_token":"at"}`}
	tok, err := clientWith(rt).GetGuestToken(Credentials{ClientID: 1, ClientSecret: "s"})
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
