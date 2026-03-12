package main

import "errors"

type exitCodeError struct {
	code int
	err  error
}

func (e *exitCodeError) Error() string {
	return e.err.Error()
}

func (e *exitCodeError) Unwrap() error {
	return e.err
}

func extractExitCode(err error) (int, bool) {
	var coded *exitCodeError
	if !errors.As(err, &coded) {
		return 0, false
	}
	if coded.code <= 0 {
		return 1, true
	}
	return coded.code, true
}
