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

// Reusable fp-go ord/eq instances and derived predicates.
var (
	// Base ord/eq instances
	IntOrd     = ORD.FromStrictCompare[int]()
	Float64Ord = ORD.FromStrictCompare[float64]()
	IntEq      = EQ.FromStrictEquals[int]()
	Float64Eq  = EQ.FromStrictEquals[float64]()
	StringEq   = EQ.FromStrictEquals[string]()

	// IntBetween(lo, hi)(x) → lo <= x < hi (exclusive upper bound)
	IntBetween = ORD.Between(IntOrd)

	// IntBetweenInclusive(lo, hi)(x) → lo <= x <= hi (inclusive upper bound)
	IntBetweenInclusive = func(lo, hi int) Pred.Predicate[int] {
		return Pred.And(ORD.Geq(IntOrd)(lo))(ORD.Leq(IntOrd)(hi))
	}

	// MinStrLen(n)(s) → len(s) >= n
	MinStrLen = F.Flow2(
		ORD.Geq(IntOrd),
		Pred.ContraMap(Str.Size),
	)

	// MaxStrLen(n)(s) → len(s) <= n
	MaxStrLen = F.Flow2(
		ORD.Leq(IntOrd),
		Pred.ContraMap(Str.Size),
	)

	// StrLenEq(n)(s) → len(s) == n
	StrLenEq = F.Flow2(
		EQ.Equals(IntEq),
		Pred.ContraMap(Str.Size),
	)

	// NotEqualF64(v)(x) → x != v
	NotEqualF64 = F.Flow2(EQ.Equals(Float64Eq), Pred.Not)

	// NotEqualInt(v)(x) → x != v
	NotEqualInt = F.Flow2(EQ.Equals(IntEq), Pred.Not)

	// NotEqualStr(v)(x) → x != v
	NotEqualStr = F.Flow2(EQ.Equals(StringEq), Pred.Not)

	// StrEq(s)(x) → x == s
	StrEq = EQ.Equals(StringEq)
)
