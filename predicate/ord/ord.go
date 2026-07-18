// Package predord provides reusable fp-go ord/eq instances and derived
// predicates.
package predord

import (
	EQ "github.com/IBM/fp-go/v2/eq"
	F "github.com/IBM/fp-go/v2/function"
	ORD "github.com/IBM/fp-go/v2/ord"
	Pred "github.com/IBM/fp-go/v2/predicate"
	Str "github.com/IBM/fp-go/v2/string"
)

var (
	// IntOrd is a strict-comparison Ord instance for int.
	IntOrd = ORD.FromStrictCompare[int]()

	// Float64Ord is a strict-comparison Ord instance for float64.
	Float64Ord = ORD.FromStrictCompare[float64]()

	// IntEq is a strict-equality Eq instance for int.
	IntEq = EQ.FromStrictEquals[int]()

	// Float64Eq is a strict-equality Eq instance for float64.
	Float64Eq = EQ.FromStrictEquals[float64]()

	// StringEq is a strict-equality Eq instance for string.
	StringEq = EQ.FromStrictEquals[string]()

	// IntBetween returns a Predicate that is true when lo <= x < hi
	// (exclusive upper bound).
	IntBetween = ORD.Between(IntOrd)

	// IntBetweenInclusive returns a Predicate that is true when
	// lo <= x <= hi (inclusive upper bound).
	IntBetweenInclusive = func(lo, hi int) Pred.Predicate[int] {
		return Pred.And(ORD.Geq(IntOrd)(lo))(ORD.Leq(IntOrd)(hi))
	}

	// MinStrLen returns a Predicate that is true when len(s) >= n.
	MinStrLen = F.Flow2(
		ORD.Geq(IntOrd),
		Pred.ContraMap(Str.Size),
	)

	// MaxStrLen returns a Predicate that is true when len(s) <= n.
	MaxStrLen = F.Flow2(
		ORD.Leq(IntOrd),
		Pred.ContraMap(Str.Size),
	)

	// StrLenEq returns a Predicate that is true when len(s) == n.
	StrLenEq = F.Flow2(
		EQ.Equals(IntEq),
		Pred.ContraMap(Str.Size),
	)

	// NotEqualF64 returns a Predicate that is true when x != v.
	NotEqualF64 = F.Flow2(EQ.Equals(Float64Eq), Pred.Not)

	// NotEqualInt returns a Predicate that is true when x != v.
	NotEqualInt = F.Flow2(EQ.Equals(IntEq), Pred.Not)

	// NotEqualStr returns a Predicate that is true when x != v.
	NotEqualStr = F.Flow2(EQ.Equals(StringEq), Pred.Not)

	// StrEq returns a Predicate that is true when x == s.
	StrEq = EQ.Equals(StringEq)
)
