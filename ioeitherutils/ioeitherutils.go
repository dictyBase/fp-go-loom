// Package ioeitherutils provides utility functions for working with IOEither.
package ioeitherutils

import (
	E "github.com/IBM/fp-go/v2/either"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)

// ToEither converts lazy IOEither to eager Either evaluation.
func ToEither[ERR, A any](
	ioe IOE.IOEither[ERR, A],
) E.Either[ERR, A] {
	return ioe()
}
