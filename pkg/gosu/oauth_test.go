package gosu

import (
	"net/url"
	"strings"
	"testing"
)

func TestAuthorizeURL(t *testing.T) {
	raw := AuthorizeURL(123, "https://app/cb", []Scope{ScopePublic, ScopeIdentify}, "csrf")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Host != "osu.ppy.sh" || u.Path != "/oauth/authorize" {
		t.Errorf("endpoint = %s%s, want osu.ppy.sh/oauth/authorize", u.Host, u.Path)
	}
	q := u.Query()
	for k, want := range map[string]string{
		"client_id":     "123",
		"redirect_uri":  "https://app/cb",
		"response_type": "code",
		"scope":         "public identify",
		"state":         "csrf",
	} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, q.Get(k), want)
		}
	}
}

func TestAuthorizeURLOmitsEmptyState(t *testing.T) {
	raw := AuthorizeURL(1, "https://app/cb", []Scope{ScopePublic}, "")
	if strings.Contains(raw, "state=") {
		t.Errorf("url %q carries a state param for empty state", raw)
	}
}
