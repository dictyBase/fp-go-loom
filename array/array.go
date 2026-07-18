// Package arrutils provides generic array utility functions built with fp-go v2 combinators.
package arrutils

import (
	Arr "github.com/IBM/fp-go/v2/array"
	F "github.com/IBM/fp-go/v2/function"
	O "github.com/IBM/fp-go/v2/option"
)

// Compact extracts Some values from a slice of Options, discarding None (catMaybes).
func Compact[A any](opts []O.Option[A]) []A {
	return F.Pipe1(opts, Arr.FilterMap(F.Identity[O.Option[A]]))
}

// ParseWith returns a function that maps []string to []A, silently discarding parse errors.
func ParseWith[A any](
	parseFn func(string) (A, error),
) func([]string) []A {
	return Arr.FilterMap(func(s string) O.Option[A] {
		return O.TryCatch(
			func() (A, error) { return parseFn(s) },
		)
	})
}
