package gosu

import (
	"errors"
	"testing"
)

func TestErrUserNotFoundWrapsStatusError(t *testing.T) {
	if !errors.Is(ErrUserNotFound, ErrUserNotFound) {
		t.Fatal("ErrUserNotFound does not match itself")
	}

	var se *StatusError
	if !errors.As(ErrUserNotFound, &se) {
		t.Fatal("ErrUserNotFound does not unwrap to a StatusError")
	}
	if se.Code != 404 {
		t.Fatalf("Code = %d, want 404", se.Code)
	}
}

func TestStatusErrorAsRecoversCode(t *testing.T) {
	err := error(&StatusError{Code: 429, Status: "429 Too Many Requests"})

	var se *StatusError
	if !errors.As(err, &se) || se.Code != 429 {
		t.Fatalf("errors.As failed to recover 429 from %v", err)
	}
}
