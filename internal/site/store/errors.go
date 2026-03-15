package store

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("store not found")
	ErrAlreadyExists = errors.New("store already exists")
	ErrClosedSession = errors.New("store session closed")
	ErrConflict      = errors.New("store conflict")
)

func notFoundError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrNotFound, fmt.Sprintf(format, args...))
}

func alreadyExistsError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrAlreadyExists, fmt.Sprintf(format, args...))
}

func closedSessionError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrClosedSession, fmt.Sprintf(format, args...))
}

func conflictError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrConflict, fmt.Sprintf(format, args...))
}
