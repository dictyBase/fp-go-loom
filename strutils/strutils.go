// Package strutils provides utility functions for string
// operations using fp-go functional patterns.
package strutils

import (
	"github.com/IBM/fp-go/v2/monoid"
	S "github.com/IBM/fp-go/v2/string"
)

// JoinStrings joins multiple strings using the fp-go string
// monoid via [A.Fold]. It is a variadic wrapper around
// folding the parts with [S.Monoid].
func JoinStrings(parts ...string) string {
	return monoid.Fold(S.Monoid)(parts)
}
