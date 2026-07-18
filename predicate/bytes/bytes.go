// Package predbytes provides predicates for []byte.
package predbytes

import (
	B "github.com/IBM/fp-go/v2/bytes"
	F "github.com/IBM/fp-go/v2/function"
	N "github.com/IBM/fp-go/v2/number"
	Pred "github.com/IBM/fp-go/v2/predicate"
)

var (

	// HasPositiveLen composes ByteSize with IsPositiveLen via ContraMap,
	// producing a Predicate[[]byte].
	HasPositiveLen = Pred.ContraMap(B.Size)(N.MoreThan(0))

	// IsNonEmpty is a human-readable alias for HasPositiveLen.
	IsNonEmpty = F.Identity(HasPositiveLen)
)
