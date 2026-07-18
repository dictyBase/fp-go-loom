// Package eithererr provides utilities for working with functions that return
// either a value or an error.
package eithererr

import "errors"

// ConstErr returns a function that ignores its argument and always returns the same
// pre-allocated errors.New(msg) instance. This means callers using errors.Is for
// sentinel matching get stable identity across calls.
func ConstErr[A any](msg string) func(A) error {
	err := errors.New(msg)
	return func(_ A) error { return err }
}
