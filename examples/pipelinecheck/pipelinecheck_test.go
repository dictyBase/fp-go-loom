package main

import (
	"strings"
	"testing"

	pipelinecheck "github.com/dictyBase/fp-go-loom/pipelinecheck"
	"github.com/stretchr/testify/require"
)

// TestPipelineContinuity is the gate itself. Drop this file plus a
// matching IsEntrypoint predicate into any package that owns CLI
// entrypoints, then run go test as usual.
func TestPipelineContinuity(t *testing.T) {
	t.Parallel()

	pipelinecheck.Require(t, pipelinecheck.Config{
		Roots: []string{"."},
		IsEntrypoint: func(name string) bool {
			return strings.HasPrefix(name, "run")
		},
		RequirePointFreeSeed:      true,
		RequirePointFreeBranching: true,
		RequireTrivialDelegate:    true,
	})

	rawCfg := pipelinecheck.Config{Roots: []string{"."}}
	pipelinecheck.RequireNoNonRawTryCatchCallback(t, rawCfg)
	pipelinecheck.RequireNoIfErrInTryCatch(t, rawCfg)
	pipelinecheck.RequireBareIf(t, rawCfg)
}

// TestGateCatchesViolation pins the violation fixture in
// testdata/violation. It proves the gate notices a single-statement
// pass-through wrapper referenced from an F.Pipe site, and that the
// gate must live in a non-test .go file to be scanned at all.
func TestGateCatchesViolation(t *testing.T) {
	t.Parallel()

	vs, err := pipelinecheck.Check(pipelinecheck.Config{
		Roots:                  []string{"testdata/violation"},
		RequireTrivialDelegate: true,
	})
	require.NoError(t, err)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, "only delegates to")
}
