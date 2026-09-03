package gosu

import (
	"net/http"
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

// refreshSkew renews a token this long before it expires, so a token handed out never dies
// mid-flight while the request is still on the wire.
const refreshSkew = time.Minute

// Token is a credential a request stamps. Satisfied by *GuestToken and
// *ResourceToken, the two grants osu! issues.
type Token interface {
	Authorization() string
	ExpiresAt() time.Time
	Expired() bool
}

// TokenSource yields the credential the general Client stamps. Its concrete
// implementations are the *TokenProvider types.
type TokenSource interface {
	Token() (Token, error)
}

// ResourceOwnerSource yields a resource-owner credential. A ResourceClient is built from
// one, which is what keeps a user-scoped call off a guest client at compile time.
type ResourceOwnerSource interface {
	ResourceToken() (*ResourceToken, error)
}

func expiresAt(obtainedAt time.Time, expiresIn int) time.Time {
	return obtainedAt.Add(time.Duration(expiresIn) * time.Second)
}

func stale(t Token) bool {
	return !time.Now().Before(t.ExpiresAt().Add(-refreshSkew))
}

// OAuth runs the osu! token grants. It is not auth-bound, so a token source can hold one to
// acquire and refresh without the circularity of needing a Client first. Grants run unpaced:
// the token endpoint is off the api/v2 ceiling.
type OAuth struct {
	creds Credentials
	http  *http.Client
}

func NewOAuth(creds Credentials, opts ...Option) *OAuth {
	cfg := buildConfig(opts)
	tr := &transport{base: cfg.base}
	return &OAuth{creds: creds, http: &http.Client{Timeout: 30 * time.Second, Transport: tr}}
}

func (o *OAuth) app() oauth.App {
	return oauth.App{ClientID: o.creds.ClientID, ClientSecret: o.creds.ClientSecret}
}

// AuthorizeURL builds the URL to send a user to so they grant the requested scopes. osu!
// redirects back to redirectURI with a code and, when set, state, which the caller uses as
// its CSRF token; this package neither mints nor checks it.
func (o *OAuth) AuthorizeURL(redirectURI string, scopes []Scope, state string) string {
	names := make([]string, len(scopes))
	for i, s := range scopes {
		names[i] = string(s)
	}

	q := url.Values{}
	q.Set("client_id", strconv.Itoa(o.creds.ClientID))
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(names, " "))
	if state != "" {
		q.Set("state", state)
	}
	return buildOAUTHUrl("authorize") + "?" + q.Encode()
}

// Guest runs the client-credentials grant, yielding a token with no resource owner.
func (o *OAuth) Guest() (*GuestToken, error) {
	token := &GuestToken{}
	if err := oauth.ClientCredentials(o.http, buildOAUTHUrl("token"), o.app(), "public", token); err != nil {
		return nil, err
	}
	token.ObtainedAt = time.Now()
	return token, nil
}

// Exchange trades the code osu! sent to the redirect for a user token. The redirectURI must
// match the one AuthorizeURL was built with.
func (o *OAuth) Exchange(code, redirectURI string) (*ResourceToken, error) {
	token := &ResourceToken{}
	if err := oauth.AuthorizationCode(o.http, buildOAUTHUrl("token"), o.app(), code, redirectURI, token); err != nil {
		return nil, err
	}
	token.ObtainedAt = time.Now()
	return token, nil
}

// Refresh trades a refresh token for a fresh user token. osu! rotates the refresh token, so
// the returned token carries a new one to store.
func (o *OAuth) Refresh(refreshToken string) (*ResourceToken, error) {
	token := &ResourceToken{}
	if err := oauth.Refresh(o.http, buildOAUTHUrl("token"), o.app(), refreshToken, token); err != nil {
		return nil, err
	}
	token.ObtainedAt = time.Now()
	return token, nil
}
