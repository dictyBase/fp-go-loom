// Package bad_fold is a negative fixture for the redundant-Fold rule.
package bad_fold

import (
	E "github.com/IBM/fp-go/v2/either"
	F "github.com/IBM/fp-go/v2/function"
)

type A struct{}

func wrapErr(e error) error { return e }

func Bad(e E.Either[error, A]) error {
	return E.Fold(wrapErr, F.Constant1[A](error(nil)))(e)
}
