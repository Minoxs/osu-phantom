package gosu

import (
	"time"

	"github.com/minoxs/gosu-api/internal/oauth"
)

// GuestToken is the client-credentials result. It has no resource owner and no
// refresh token; refreshing it means running the grant again.
type GuestToken struct {
	AccessToken string    `json:"access_token"`
	ExpiresIn   int       `json:"expires_in"`
	TokenType   string    `json:"token_type"`
	ObtainedAt  time.Time `json:"-"`
}

func (t *GuestToken) Authorization() string { return createHeader(t.TokenType, t.AccessToken) }
func (t *GuestToken) ExpiresAt() time.Time  { return expiresAt(t.ObtainedAt, t.ExpiresIn) }
func (t *GuestToken) Expired() bool         { return !time.Now().Before(t.ExpiresAt()) }

// GetGuestToken runs the client-credentials grant, yielding a token with no
// resource owner. It is the token the read endpoints use.
func (c *Client) GetGuestToken(creds Credentials) (*GuestToken, error) {
	token := &GuestToken{}
	if err := oauth.ClientCredentials(c.http, buildOAUTHUrl("token"), app(creds), "public", token); err != nil {
		return nil, err
	}
	token.ObtainedAt = time.Now()
	return token, nil
}
