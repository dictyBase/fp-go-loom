// Package main is a runnable example of a pipelinecheck gate over a
// compliant fp-go CLI entrypoint. Copy the shape, not the code.
package main

import (
	"context"
	"fmt"

	E "github.com/IBM/fp-go/v2/either"
	F "github.com/IBM/fp-go/v2/function"
	IOE "github.com/IBM/fp-go/v2/ioeither"
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

// runDemo is the CLI entrypoint. The whole body is one F.Pipe
// expression. Locals carry the boundary values into the raw effect,
// so the seed references no entrypoint parameter directly.
func runDemo(in demoInput) IOE.IOEither[error, string] {
	ctx, fetch, name := in.ctx, in.fetch, in.name

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
	in := demoInput{
		ctx:   context.Background(),
		fetch: func(_ context.Context, s string) (string, error) { return s, nil },
		name:  "pipelinecheck",
	}
	res := ioeutils.ToEither(runDemo(in))
	fmt.Println(E.UnwrapError(res))
}
