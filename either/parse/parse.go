// Package eitherparse provides functions to parse strings into various types,
// returning Either[error, T] to handle parsing errors gracefully.
package eitherparse

import (
	"strconv"

	E "github.com/IBM/fp-go/v2/either"
	fperrors "github.com/IBM/fp-go/v2/errors"
	F "github.com/IBM/fp-go/v2/function"
)

// ParseInt converts a string to an int, returning Either[error, int].
// On failure the left contains an error prefixed with "failed to parse int".
var ParseInt = F.Flow2(
	E.Eitherize1(strconv.Atoi),
	E.MapLeft[int](fperrors.OnError("failed to parse int")),
)
