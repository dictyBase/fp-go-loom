// Package matchopt provides helpers for building Option-based pattern-match arms.
package matchopt

import (
	arr "github.com/IBM/fp-go/v2/array"
	F "github.com/IBM/fp-go/v2/function"
	O "github.com/IBM/fp-go/v2/option"
)

// Case builds a pattern arm: if pred holds, apply f and wrap in Some; else None.
func Case[A, B any](
	pred func(A) bool,
	f func(A) B,
) func(A) O.Option[B] {
	return F.Flow2(O.FromPredicate(pred), O.Map(f))
}

// Const builds a pattern arm that yields a constant value when pred holds.
func Const[A, B any](
	pred func(A) bool,
	value B,
) func(A) O.Option[B] {
	return Case(pred, func(_ A) B { return value })
}

// Default builds a catch-all arm that always matches and applies f.
func Default[A, B any](f func(A) B) func(A) O.Option[B] {
	return Case(func(_ A) bool { return true }, f)
}

// Alt returns the first Some in cases, or None if all are None.
func Alt[A any](cases []O.Option[A]) O.Option[A] {
	return O.AltAllArray(O.None[A]())(cases)
}

// First returns the value of the first Some in cases, or fallback.
func First[A any](fallback A, cases []O.Option[A]) A {
	return F.Pipe1(Alt(cases), O.GetOrElse(F.Constant(fallback)))
}

// Match lazily dispatches input to the first matching case in order,
// returning fallback when no case matches. Unlike First, which folds
// already-applied options, Match applies cases itself and stops invoking
// them at the first Some: later cases never run, so guard cost is not
// paid past the match.
func Match[A, B any](
	fallback B,
	cases []O.Kleisli[A, B],
) func(A) B {
	return F.Flow3(
		F.Flip(applyCase[A, B]),
		F.Flip(arr.FindFirstMap[O.Kleisli[A, B], B])(cases),
		O.GetOrElse(F.Constant(fallback)),
	)
}

// applyCase curries a Kleisli case so F.Flip can fix the input.
func applyCase[A, B any](
	c O.Kleisli[A, B],
) func(A) O.Option[B] {
	return c
}
