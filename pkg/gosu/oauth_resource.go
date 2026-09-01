package gosu

import (
	"errors"
	"sync"
	"time"
)

// ErrNoResourceToken is returned by a resource source asked for a token it does not hold.
var ErrNoResourceToken = errors.New("no resource token to refresh")

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

// ResourceOwnerTokenProvider is a ResourceOwnerSource backed by the refresh grant. It holds
// the token from an exchange and, once it nears expiry, refreshes it, storing the rotated
// refresh token osu! returns.
type ResourceOwnerTokenProvider struct {
	oauth *OAuth

	mu  sync.Mutex
	tok *ResourceToken
}

func NewResourceOwnerTokenProvider(o *OAuth, initial *ResourceToken) *ResourceOwnerTokenProvider {
	return &ResourceOwnerTokenProvider{oauth: o, tok: initial}
}

func (p *ResourceOwnerTokenProvider) ResourceToken() (*ResourceToken, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.tok == nil {
		return nil, ErrNoResourceToken
	}
	if !stale(p.tok) {
		return p.tok, nil
	}
	refreshed, err := p.oauth.Refresh(p.tok.RefreshToken)
	if err != nil {
		return nil, err
	}
	p.tok = refreshed
	return p.tok, nil
}

// StaticResourceOwnerTokenProvider is a ResourceOwnerSource holding a token the caller
// manages. SetToken replaces it without rebuilding the client; it never refreshes on its own.
type StaticResourceOwnerTokenProvider struct {
	mu  sync.RWMutex
	tok *ResourceToken
}

func NewStaticResourceOwnerTokenProvider(tok *ResourceToken) *StaticResourceOwnerTokenProvider {
	return &StaticResourceOwnerTokenProvider{tok: tok}
}

func (p *StaticResourceOwnerTokenProvider) ResourceToken() (*ResourceToken, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tok, nil
}

func (p *StaticResourceOwnerTokenProvider) SetToken(tok *ResourceToken) {
	p.mu.Lock()
	p.tok = tok
	p.mu.Unlock()
}
