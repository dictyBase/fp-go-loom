package pipelinecheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	wantLocalClosureMsg = "local closure or combinator assigned inside function"

	localClosurePreamble = `package testpkg
import (
	"errors"
	A "github.com/IBM/fp-go/v2/array"
	E "github.com/IBM/fp-go/v2/either"
	F "github.com/IBM/fp-go/v2/function"
	IOE "github.com/IBM/fp-go/v2/ioeither"
	O "github.com/IBM/fp-go/v2/option"
	S "github.com/IBM/fp-go/v2/string"
)
func exec(s string) IOE.IOEither[error, string] {
	return IOE.Of[error](s)
}
func toUpper(s string) string { return s }
`
)

var localClosureCases = []struct {
	name      string
	body      string
	wantCount int
}{
	{
		name: "local func literal assigned inside function flagged",
		body: `func bad(xs []string) []string {
	toInput := func(s string) string { return s }
	return F.Pipe1(xs, A.Map(toInput))
}`,
		wantCount: 1,
	},
	{
		name: "local F.Flow assigned inside function flagged",
		body: `func bad(opt O.Option[string]) E.Either[error, string] {
	extractRoot := F.Flow2(
		O.Map(toUpper),
		O.Chain(O.FromPredicate(S.IsNonEmpty)),
	)
	nullErr := F.Constant(errors.New("null"))
	return F.Pipe2(
		opt,
		extractRoot,
		E.FromOption[string](nullErr),
	)
}`,
		wantCount: 2,
	},
	{
		name: "split F.Pipe seed and local TraverseArray flagged",
		body: `func bad(xs []string) IOE.IOEither[error, []string] {
	inputs := F.Pipe1(xs, A.Map(toUpper))
	traverseProbes := IOE.TraverseArray(exec)
	return F.Pipe1(inputs, traverseProbes)
}`,
		wantCount: 2,
	},
	{
		name: "var declaration with func literal inside function flagged",
		body: `func bad(xs []string) []string {
	var fn = func(s string) string { return s }
	return F.Pipe1(xs, A.Map(fn))
}`,
		wantCount: 1,
	},
	{
		name: "closure inside TryCatchError callback allowed",
		body: `func good() IOE.IOEither[error, string] {
	return IOE.TryCatchError(func() (string, error) {
		fn := func() string { return "ok" }
		return fn(), nil
	})
}`,
		wantCount: 0,
	},
	{
		name: "package-level var and flat F.PipeN clean",
		body: `var (
	traverseAll = IOE.TraverseArray(exec)
	mapUpper    = A.Map(toUpper)
)
func good(xs []string) IOE.IOEither[error, []string] {
	return F.Pipe2(
		xs,
		mapUpper,
		traverseAll,
	)
}`,
		wantCount: 0,
	},
}

func TestLocalPipeClosure_Table(t *testing.T) {
	for _, tc := range localClosureCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, localClosurePreamble+tc.body)
			vs := checkNoLocalPipeClosure(
				fset,
				f,
				DefaultAllowLocalPipeClosureDirective,
			)
			require.Len(t, vs, tc.wantCount)
			if tc.wantCount > 0 {
				require.Contains(
					t,
					vs[0].Message,
					wantLocalClosureMsg,
				)
			}
		})
	}
}

func TestLocalPipeClosure_FileWithoutFpGoIgnored(t *testing.T) {
	fset, f := parse(t, `package testpkg
func f() string {
	g := func() string { return "ok" }
	return g()
}`)
	require.Empty(t, checkNoLocalPipeClosure(
		fset,
		f,
		DefaultAllowLocalPipeClosureDirective,
	))
}

func TestLocalPipeClosure_AllowDirective(t *testing.T) {
	fset, f := parse(t, localClosurePreamble+`
// fp-go:allow-local-pipe-closure recursive local helper
func ok(xs []string) []string {
	toInput := func(s string) string { return s }
	return F.Pipe1(xs, A.Map(toInput))
}`)
	require.Empty(t, checkNoLocalPipeClosure(
		fset,
		f,
		DefaultAllowLocalPipeClosureDirective,
	))
}

func TestLocalPipeClosure_DirectiveWithoutReason(t *testing.T) {
	fset, f := parse(t, localClosurePreamble+`
// fp-go:allow-local-pipe-closure
func ok(xs []string) []string {
	toInput := func(s string) string { return s }
	return F.Pipe1(xs, A.Map(toInput))
}`)
	vs := checkNoLocalPipeClosure(
		fset,
		f,
		DefaultAllowLocalPipeClosureDirective,
	)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, "non-empty reason")
}

func TestLocalPipeClosure_CheckAndRequireOverRoots(
	t *testing.T,
) {
	dir := writeFixture(t, localClosurePreamble+`
func bad(xs []string) []string {
	toInput := func(s string) string { return s }
	return F.Pipe1(xs, A.Map(toInput))
}`)
	cfg := Config{Roots: []string{dir}}

	vs, err := CheckNoLocalPipeClosure(cfg)
	require.NoError(t, err)
	require.Len(t, vs, 1)

	rec := &fakeReporter{}
	RequireNoLocalPipeClosure(rec, cfg)
	require.Len(t, rec.errors, 1)
	require.Contains(t, rec.errors[0], wantLocalClosureMsg)
}

func TestLocalPipeClosure_ScanErrorIsFatal(t *testing.T) {
	rec := &fakeReporter{}
	RequireNoLocalPipeClosure(rec, Config{Roots: []string{
		t.TempDir() + "/missing",
	}})
	require.Contains(t, rec.fatal, "pipelinecheck:")
}
