package gosu

import (
	"fmt"
	"net/http"
)

// StatusError is returned when the osu! API answers with an unmapped non-200 status.
type StatusError struct {
	Code   int
	Status string
}

func (e *StatusError) Error() string { return e.Status }

// ErrUserNotFound is returned by profile lookups when the osu! API has no such user.
var ErrUserNotFound = fmt.Errorf("user not found: %w", &StatusError{Code: http.StatusNotFound, Status: "404 Not Found"})
