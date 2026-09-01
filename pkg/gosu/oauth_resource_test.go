package gosu

import (
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

func TestExchangeCode(t *testing.T) {
	rt := &roundTripFunc{body: `{"token_type":"Bearer","expires_in":86400,"access_token":"at","refresh_token":"rt"}`}
	tok, err := clientWith(rt).ExchangeCode(Credentials{ClientID: 1, ClientSecret: "s"}, "code", "https://app/cb")
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

func TestRefreshTokenLeavesInputUntouched(t *testing.T) {
	rt := &roundTripFunc{body: `{"token_type":"Bearer","expires_in":86400,"access_token":"new","refresh_token":"rotated"}`}
	old := &ResourceToken{AccessToken: "old", RefreshToken: "original"}
	got, err := clientWith(rt).RefreshToken(Credentials{ClientID: 1, ClientSecret: "s"}, old)
	if err != nil {
		t.Fatal(err)
	}
	if got.RefreshToken != "rotated" || got.AccessToken != "new" {
		t.Errorf("refreshed = %+v, want access new, refresh rotated", got)
	}
	if old.RefreshToken != "original" {
		t.Errorf("input mutated, RefreshToken = %q, want original", old.RefreshToken)
	}
}
