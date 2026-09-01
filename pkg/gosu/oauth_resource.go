package gosu

import (
	"time"

	"github.com/minoxs/gosu-api/internal/oauth"
)

// ResourceToken is the authorization-code result, attached to a logged-in user.
// It carries a refresh token the refresh grant rotates.
type ResourceToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	TokenType    string    `json:"token_type"`
	ObtainedAt   time.Time `json:"-"`
}

func (t *ResourceToken) Authorization() string { return createHeader(t.TokenType, t.AccessToken) }
func (t *ResourceToken) ExpiresAt() time.Time  { return expiresAt(t.ObtainedAt, t.ExpiresIn) }
func (t *ResourceToken) Expired() bool         { return !time.Now().Before(t.ExpiresAt()) }

// ExchangeCode trades the code osu! sent to the redirect for a user token. The
// redirectURI must match the one AuthorizeURL was built with.
func (c *Client) ExchangeCode(creds Credentials, code, redirectURI string) (*ResourceToken, error) {
	token := &ResourceToken{}
	if err := oauth.AuthorizationCode(c.http, buildOAUTHUrl("token"), app(creds), code, redirectURI, token); err != nil {
		return nil, err
	}
	token.ObtainedAt = time.Now()
	return token, nil
}

// RefreshToken trades a user token's refresh token for a fresh one. osu! rotates
// the refresh token, so the returned token carries a new one to store; the input
// is left untouched.
func (c *Client) RefreshToken(creds Credentials, token *ResourceToken) (*ResourceToken, error) {
	refreshed := &ResourceToken{}
	if err := oauth.Refresh(c.http, buildOAUTHUrl("token"), app(creds), token.RefreshToken, refreshed); err != nil {
		return nil, err
	}
	refreshed.ObtainedAt = time.Now()
	return refreshed, nil
}
