package strutils

import (
	"strings"

	F "github.com/IBM/fp-go/v2/function"
	M "github.com/IBM/fp-go/v2/monoid"
	O "github.com/IBM/fp-go/v2/option"
	S "github.com/IBM/fp-go/v2/string"
)

// Separator monoids. Each one joins two strings with its
// separator and is built on [S.IntersperseMonoid], so an empty
// operand drops out together with its separator:
//
//	Comma.Concat("a", "b") // "a, b"
//	Comma.Concat("a", "")  // "a"
//
// That makes an optional part (a missing DOI, an absent label)
// disappear with no branch. Empty() is "".
var (
	// Space joins with " ".
	Space = S.IntersperseMonoid(" ")

	// Comma joins with ", ".
	Comma = S.IntersperseMonoid(", ")

	// Semicolon joins with "; ".
	Semicolon = S.IntersperseMonoid("; ")

	// Colon joins with ":" and no space, as in "74:1-10".
	Colon = S.IntersperseMonoid(":")

	// Label joins a label and a value with ": ".
	Label = S.IntersperseMonoid(": ")

	// MiddleDot joins with " · ".
	MiddleDot = S.IntersperseMonoid(" · ")

	// EmDash joins with " — ".
	EmDash = S.IntersperseMonoid(" — ")

	// Newline joins with "\n".
	Newline = S.IntersperseMonoid("\n")
)

// Folds over the separator monoids. Each one joins a slice and
// skips empty elements together with their separators.
var (
	// JoinSpace joins a slice with " ".
	JoinSpace = M.ConcatAll(Space)

	// JoinComma joins a slice with ", ".
	JoinComma = M.ConcatAll(Comma)

	// JoinSemicolon joins a slice with "; ".
	JoinSemicolon = M.ConcatAll(Semicolon)

	// JoinMiddleDot joins a slice with " · ".
	JoinMiddleDot = M.ConcatAll(MiddleDot)

	// JoinLines joins a slice with "\n".
	JoinLines = M.ConcatAll(Newline)
)

var (
	// CollapseWhitespace trims s and replaces every run of
	// whitespace, including newlines, with one space.
	CollapseWhitespace = F.Flow2(strings.Fields, JoinSpace)

	// Parens wraps s in "(" and ")".
	Parens = Wrap("(", ")")

	// Brackets wraps s in "[" and "]".
	Brackets = Wrap("[", "]")

	// Quote wraps s in double quotes. It does not escape s.
	Quote = Wrap(`"`, `"`)

	// Period appends ".".
	Period = S.Append(".")
)

// Wrap returns a function that puts prefix before and suffix
// after its argument.
func Wrap(prefix, suffix string) func(string) string {
	return F.Flow2(S.Prepend(prefix), S.Append(suffix))
}

// MapNonEmpty applies f to a non-empty string and returns "" for
// an empty one. Combine it with a separator monoid to render an
// optional part: the empty result drops out with its separator.
//
//	link := MapNonEmpty(toDOILink)
//	MiddleDot.Concat(pubmed, link(doi))
func MapNonEmpty(f func(string) string) func(string) string {
	return F.Flow3(
		O.FromPredicate(S.IsNonEmpty),
		O.Map(f),
		O.GetOrElse(F.Constant(S.Monoid.Empty())),
	)
}

// ConcatFields lifts m to functions with [M.FunctionMonoid] and
// folds the projections into one function. Each projection
// renders one field of A, and m joins the results:
//
//	name := ConcatFields[Author](Space)(
//		[]func(Author) string{lastName, initials},
//	)
//	name(Author{"Smith", "A"}) // "Smith A"
func ConcatFields[A any](
	m M.Monoid[string],
) func([]func(A) string) func(A) string {
	return M.ConcatAll(M.FunctionMonoid[A](m))
}
