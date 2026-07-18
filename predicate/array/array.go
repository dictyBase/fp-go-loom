// Package predarrays provides generic slice-length predicates built with fp-go v2 combinators.
package predarrays

import (
	A "github.com/IBM/fp-go/v2/array"
	EQ "github.com/IBM/fp-go/v2/eq"
	F "github.com/IBM/fp-go/v2/function"
	N "github.com/IBM/fp-go/v2/number"
	ORD "github.com/IBM/fp-go/v2/ord"
	Pred "github.com/IBM/fp-go/v2/predicate"
	predord "github.com/dictyBase/fp-go-loom/predicate/ord"
)

// IsNonEmpty returns a Predicate[[]E] that is true when len(s) > 0.
func IsNonEmpty[E any]() Pred.Predicate[[]E] {
	return F.Pipe1(N.MoreThan(0), Pred.ContraMap(A.Size[E]))
}

// MinLen returns a Predicate[[]E] that is true when len(s) >= n.
func MinLen[E any](n int) Pred.Predicate[[]E] {
	return F.Pipe2(
		n,
		ORD.Geq(predord.IntOrd),
		Pred.ContraMap(A.Size[E]),
	)
}

// MaxLen returns a Predicate[[]E] that is true when len(s) <= n.
func MaxLen[E any](n int) Pred.Predicate[[]E] {
	return F.Pipe2(
		n,
		ORD.Leq(predord.IntOrd),
		Pred.ContraMap(A.Size[E]),
	)
}

// LenEq returns a Predicate[[]E] that is true when len(s) == n.
func LenEq[E any](n int) Pred.Predicate[[]E] {
	return F.Pipe2(
		n,
		EQ.Equals(predord.IntEq),
		Pred.ContraMap(A.Size[E]),
	)
}
