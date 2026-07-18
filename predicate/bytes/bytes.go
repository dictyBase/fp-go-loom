// Package predbytes provides predicates for []byte.
package predbytes

import (
	B "github.com/IBM/fp-go/v2/bytes"
	F "github.com/IBM/fp-go/v2/function"
	N "github.com/IBM/fp-go/v2/number"
	Pred "github.com/IBM/fp-go/v2/predicate"
)

var (
	// HasPositiveLen returns a Predicate that is true when len(b) > 0,
	// composing ByteSize with MoreThan(0) via ContraMap.
	HasPositiveLen = Pred.ContraMap(B.Size)(N.MoreThan(0))

	// IsNonEmpty is a human-readable alias for HasPositiveLen.
	IsNonEmpty = F.Identity(HasPositiveLen)
)
