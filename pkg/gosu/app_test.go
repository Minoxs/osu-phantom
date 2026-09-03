package gosu

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// dispatchRT serves the guest grant on the token endpoint and a canned body elsewhere,
// recording the Authorization the api request carried.
type dispatchRT struct {
	token   string
	api     string
	apiAuth string
}

func (d *dispatchRT) RoundTrip(req *http.Request) (*http.Response, error) {
	body := d.api
	if strings.HasSuffix(req.URL.Path, "/oauth/token") {
		body = d.token
	} else {
		d.apiAuth = req.Header.Get(authHeader)
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func TestAppGuestClientStampsGuestTokenThroughTransport(t *testing.T) {
	rt := &dispatchRT{
		token: `{"token_type":"Bearer","expires_in":86400,"access_token":"guest-at"}`,
		api:   `{"id":1,"username":"peppy"}`,
	}
	app := NewApp(1, "s", Transport(rt))

	u, err := app.GuestClient(0).GetUser(1)
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "peppy" {
		t.Errorf("username = %q, want peppy", u.Username)
	}
	if rt.apiAuth != "Bearer guest-at" {
		t.Errorf("api Authorization = %q, want Bearer guest-at", rt.apiAuth)
	}
}

func TestAppResourceOwnerClientStampsUserToken(t *testing.T) {
	rt := &dispatchRT{api: `{"id":2,"username":"me"}`}
	app := NewApp(1, "s", Transport(rt))
	tok := &ResourceToken{TokenType: "Bearer", AccessToken: "user-at", ExpiresIn: 3600, ObtainedAt: time.Now()}

	if _, err := app.ResourceOwnerClient(0, tok).GetOwnUser(); err != nil {
		t.Fatal(err)
	}
	if rt.apiAuth != "Bearer user-at" {
		t.Errorf("api Authorization = %q, want Bearer user-at", rt.apiAuth)
	}
}

func TestAppClientsShareOneLimiter(t *testing.T) {
	app := NewApp(1, "s")
	if app.GuestClient(0).http.Transport.(*transport).limiter != app.ResourceOwnerClient(0, &ResourceToken{}).http.Transport.(*transport).limiter {
		t.Error("guest and resource clients do not share the app limiter")
	}
}
