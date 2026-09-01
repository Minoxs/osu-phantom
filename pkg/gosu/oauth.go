package gosu

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/minoxs/gosu-api/internal/oauth"
)

// Scope is an osu! OAuth scope named in the authorize step.
type Scope string

const (
	ScopePublic          Scope = "public"
	ScopeIdentify        Scope = "identify"
	ScopeFriendsRead     Scope = "friends.read"
	ScopeChatRead        Scope = "chat.read"
	ScopeChatWrite       Scope = "chat.write"
	ScopeChatWriteManage Scope = "chat.write_manage"
	ScopeForumWrite      Scope = "forum.write"
	// ScopeDelegate acts as the owner of the client and is client-credentials only.
	ScopeDelegate Scope = "delegate"
)

// Token is a credential a request stamps. Satisfied by *GuestToken and
// *ResourceToken, the two grants osu! issues.
type Token interface {
	Authorization() string
	ExpiresAt() time.Time
	Expired() bool
}

func expiresAt(obtainedAt time.Time, expiresIn int) time.Time {
	return obtainedAt.Add(time.Duration(expiresIn) * time.Second)
}

// AuthorizeURL builds the URL to send a user to so they grant the requested
// scopes. osu! redirects back to redirectURI with a code and, when set, state,
// which the caller uses as its CSRF token; this package neither mints nor checks it.
func AuthorizeURL(clientID int, redirectURI string, scopes []Scope, state string) string {
	names := make([]string, len(scopes))
	for i, s := range scopes {
		names[i] = string(s)
	}

	q := url.Values{}
	q.Set("client_id", strconv.Itoa(clientID))
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(names, " "))
	if state != "" {
		q.Set("state", state)
	}
	return buildOAUTHUrl("authorize") + "?" + q.Encode()
}

func app(creds Credentials) oauth.App {
	return oauth.App{ClientID: creds.ClientID, ClientSecret: creds.ClientSecret}
}
