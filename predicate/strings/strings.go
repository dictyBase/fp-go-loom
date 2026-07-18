// Package predstrings provides predicates on strings, derived from curried
// stdlib string functions.
package predstrings

import (
	"strings"

	F "github.com/IBM/fp-go/v2/function"
	ORD "github.com/IBM/fp-go/v2/ord"
	Pred "github.com/IBM/fp-go/v2/predicate"
	Str "github.com/IBM/fp-go/v2/string"
	predord "github.com/dictyBase/fp-go-loom/predicate/ord"
)

var (
	// LastIndexOf returns a function that reports the index of the
	// last occurrence of substr in s (-1 if absent).
	LastIndexOf = F.Bind2of2(strings.LastIndex)

	// HasSuffix returns a function that reports whether s ends with suffix.
	HasSuffix = F.Bind2of2(strings.HasSuffix)

	// ContainsRuneClass returns a function that reports whether s
	// contains any rune satisfying pred.
	ContainsRuneClass = F.Bind2of2(strings.ContainsFunc)

	// HasAtSign returns true when "@" exists anywhere in the string
	// (last index >= 0).
	HasAtSign = F.Pipe2(
		0,
		ORD.Geq(predord.IntOrd),
		Pred.ContraMap(LastIndexOf("@")),
	)
)

// StrLenBetween returns a Predicate that is true when len(s) is
// between first and second (inclusive).
func StrLenBetween(first, second int) Pred.Predicate[string] {
	return Pred.ContraMap(
		Str.Size,
	)(
		predord.IntBetweenInclusive(first, second))
}
