// Package main is a runnable example of a pipelinecheck gate over a
// compliant fp-go CLI entrypoint. Copy the shape, not the code.
package main

import (
	"context"
	"fmt"

	E "github.com/IBM/fp-go/v2/either"
	F "github.com/IBM/fp-go/v2/function"
	IOE "github.com/IBM/fp-go/v2/ioeither"
	L "github.com/IBM/fp-go/v2/optics/lens"

	ioeutils "github.com/dictyBase/fp-go-loom/ioeitherutils"
)

// demoInput is the boundary record for the entrypoint. The fetch
// function is the raw effect adapter; ctx sits at the edge for
// cancellation.
type demoInput struct {
	ctx   context.Context
	fetch func(context.Context, string) (string, error)
	name  string
}

var (
	demoCtxLens = L.MakeLens(
		func(in demoInput) context.Context { return in.ctx },
		func(in demoInput, v context.Context) demoInput {
			in.ctx = v
			return in
		},
	)
	demoFetchLens = L.MakeLens(
		func(
			in demoInput,
		) func(context.Context, string) (string, error) {
			return in.fetch
		},
		func(
			in demoInput,
			v func(context.Context, string) (string, error),
		) demoInput {
			in.fetch = v
			return in
		},
	)
	demoNameLens = L.MakeLens(
		func(in demoInput) string { return in.name },
		func(in demoInput, v string) demoInput {
			in.name = v
			return in
		},
	)
)

// runDemo is the CLI entrypoint. The whole body is one F.Pipe
// expression. Locals carry the boundary values into the raw effect,
// so the seed references no entrypoint parameter directly.
func runDemo(in demoInput) IOE.IOEither[error, string] {
	ctx := demoCtxLens.Get(in)
	fetch := demoFetchLens.Get(in)
	name := demoNameLens.Get(in)

	return F.Pipe2(
		IOE.TryCatchError(func() (string, error) {
			return fetch(ctx, name)
		}),
		IOE.MapLeft[string](func(err error) error {
			return fmt.Errorf("error running demo: %w", err)
		}),
		IOE.Map[error](func(s string) string {
			return fmt.Sprintf("hello %s", s)
		}),
	)
}

func main() {
	in := F.Pipe3(
		demoInput{},
		demoCtxLens.Set(context.Background()),
		demoFetchLens.Set(
			func(_ context.Context, s string) (string, error) {
				return s, nil
			},
		),
		demoNameLens.Set("pipelinecheck"),
	)
	res := ioeutils.ToEither(runDemo(in))
	fmt.Println(E.UnwrapError(res))
}
