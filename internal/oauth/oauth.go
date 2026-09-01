// Package oauth builds and sends the osu! token requests. It holds the grant
// request bodies and the POST plumbing so the public package exposes only the
// three token calls, not the shapes behind them.
package oauth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
)

const mimeJSON = "application/json"

// App identifies the OAuth client the grant is made for.
type App struct {
	ClientID     int
	ClientSecret string
}

type clientCredentialsGrant struct {
	ClientID     int    `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
	Scope        string `json:"scope"`
}

type authCodeGrant struct {
	ClientID     int    `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
}

type refreshGrant struct {
	ClientID     int    `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
}

// ClientCredentials runs the client-credentials grant, decoding the token into out.
func ClientCredentials(client *http.Client, tokenURL string, app App, scope string, out any) error {
	return post(client, tokenURL, clientCredentialsGrant{
		ClientID:     app.ClientID,
		ClientSecret: app.ClientSecret,
		GrantType:    "client_credentials",
		Scope:        scope,
	}, out)
}

// AuthorizationCode trades an authorization code for a token, decoding it into out.
func AuthorizationCode(client *http.Client, tokenURL string, app App, code, redirectURI string, out any) error {
	return post(client, tokenURL, authCodeGrant{
		ClientID:     app.ClientID,
		ClientSecret: app.ClientSecret,
		GrantType:    "authorization_code",
		Code:         code,
		RedirectURI:  redirectURI,
	}, out)
}

// Refresh trades a refresh token for a fresh token, decoding it into out.
func Refresh(client *http.Client, tokenURL string, app App, refreshToken string, out any) error {
	return post(client, tokenURL, refreshGrant{
		ClientID:     app.ClientID,
		ClientSecret: app.ClientSecret,
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
	}, out)
}

func post(client *http.Client, tokenURL string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mimeJSON)

	r, err := client.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return errors.New("status_code=" + r.Status)
	}
	return json.NewDecoder(r.Body).Decode(out)
}
