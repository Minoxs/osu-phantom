package gosu

import (
	"sync"
	"time"
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

// GuestTokenProvider is a TokenSource backed by the client-credentials grant. It caches the
// token and re-runs the grant once the cached one nears expiry.
type GuestTokenProvider struct {
	oauth *OAuth

	mu  sync.Mutex
	tok *GuestToken
}

func NewGuestTokenProvider(o *OAuth) *GuestTokenProvider {
	return &GuestTokenProvider{oauth: o}
}

func (p *GuestTokenProvider) Token() (Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.tok != nil && !stale(p.tok) {
		return p.tok, nil
	}
	tok, err := p.oauth.Guest()
	if err != nil {
		return nil, err
	}
	p.tok = tok
	return p.tok, nil
}

// StaticTokenProvider is a TokenSource holding a token the caller manages. SetToken replaces
// it without rebuilding the client; it never refreshes on its own.
type StaticTokenProvider struct {
	mu  sync.RWMutex
	tok Token
}

func NewStaticTokenProvider(tok Token) *StaticTokenProvider {
	return &StaticTokenProvider{tok: tok}
}

func (p *StaticTokenProvider) Token() (Token, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tok, nil
}

func (p *StaticTokenProvider) SetToken(tok Token) {
	p.mu.Lock()
	p.tok = tok
	p.mu.Unlock()
}
