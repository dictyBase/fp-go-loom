# pipelinecheck example

A runnable example of the mechanical fp-go pipeline-shape gate. Copy
this recipe into any Go project that consumes `github.com/IBM/fp-go/v2`.

## The recipe

1. Add the dependency:

   ```sh
   go get github.com/dictyBase/fp-go-loom/pipelinecheck
   ```

2. Drop a `pipelinecheck_test.go` into each package that owns CLI
   entrypoints or fp-go pipelines. Copy `pipelinecheck_test.go` here
   and adjust the `IsEntrypoint` predicate to your `run*` function
   names. The raw checks (`RequireNoNonRawTryCatchCallback`,
   `RequireNoIfErrInTryCatch`, `RequireBareIf`) can reach sibling
   packages by listing their paths in `Roots`.

3. Run `go test ./...`. Violations fail the test with a file position
   and a message.

## Rules the gate enforces

- `F.Pipe1(seed, namedFn)` one-step handoffs in entrypoints.
- Entrypoint seeds that pre-apply parameters
  (`F.PipeN(seedX(ctx, c), ...)` instead of a struct boundary record).
- Bare `if` statements inside fp-go pipeline callbacks.
- Hand-rolled `if err != nil` inside TryCatch callbacks.
- Single-statement pass-through wrappers referenced from
  `F.Pipe`/`F.Flow`/`IOE.*` sites.

Exemptions are doc-comment directives with a non-empty reason, for
example `// fp-go:allow-trivial-delegate <reason>`.

## What the example contains

- `main.go` — a compliant entrypoint: one `F.Pipe` expression per
  entrypoint, struct boundary seed, `IOE.TryCatchError` + `IOE.MapLeft`
  shape.
- `pipelinecheck_test.go` — the gate test plus
  `TestGateCatchesViolation`.
- `testdata/violation/violation.go` — a deliberate trivial-delegate
  violation. The test asserts `pipelinecheck.Check` reports it.

Note that `pipelinecheck` scans only non-test `.go` files. A violation
fixture must therefore live in a regular file, such as
`testdata/violation/violation.go`, not in a `_test.go` file.

## Working fixture (verified)

To prove a new gate actually fires, create a temporary non-test file
in the gated package, expect `go test` to fail, then delete it:

```go
package datasource

import (
	F "github.com/IBM/fp-go/v2/function"
)

func gateWrap(s string) string {
	return gateHelper(s)
}

func gatePipeline(s string) string {
	return F.Pipe1(s, gateWrap)
}

func gateHelper(s string) string {
	return s + "!"
}
```

Expected failure: `wrapper gateWrap only delegates to ...`.
