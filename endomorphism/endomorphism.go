// Package endoutils provides builders on top of fp-go v2
// endomorphisms.
package endoutils

import (
	A "github.com/IBM/fp-go/v2/array"
	En "github.com/IBM/fp-go/v2/endomorphism"
	F "github.com/IBM/fp-go/v2/function"
)

// BuildFrom returns a builder that makes a T from one input.
//
// Each recipe reads a value from the input and returns a
// setter (an Endomorphism[T]), usually a curried lens Set.
// The builder gives the input to every recipe, then applies
// the setters in list order to the zero value of T.
//
// The builder always starts from the zero T. It does not
// update an existing value. The recipes must be pure and
// cannot fail. Use E.Bind or IOE.Bind for fallible or
// dependent fields. When two recipes set the same field,
// the last recipe wins.
//
//	paramsOf := endoutils.BuildFrom(
//		[]func(*cli.Context) En.Endomorphism[Params]{
//			F.Flow2(inputFlag, paramsL.Input.Set),
//			F.Flow2(outputFlag, paramsL.Output.Set),
//		},
//	)
func BuildFrom[In, T any](
	edits []func(In) En.Endomorphism[T],
) func(In) T {
	return F.Flow2(
		F.Bind1st(
			A.MonadFlap[En.Endomorphism[T], In],
			edits,
		),
		En.Reduce[T],
	)
}
