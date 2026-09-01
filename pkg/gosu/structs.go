package gosu

import "errors"

// ErrUserNotFound is returned by profile lookups when the osu API has no such user.
var ErrUserNotFound = errors.New("user not found")

func createHeader(authType, authValue string) string {
	return authType + " " + authValue
}

type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}
