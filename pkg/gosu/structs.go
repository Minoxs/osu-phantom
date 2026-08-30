package gosu

import "errors"

// ErrUserNotFound is returned by profile lookups when the osu API has no such user.
var ErrUserNotFound = errors.New("user not found")

func createHeader(authType, authValue string) string {
	return authType + " " + authValue
}

type GuestToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type AuthGrant struct {
	ClientID     int    `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
	Scope        string `json:"scope"`
}
