package pipelinecheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// parse parses src as test.go and returns the file plus its fset, so
// table cases stay one line each.
func parse(
	t *testing.T,
	src string,
) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(
		fset,
		"test.go",
		src,
		parser.ParseComments,
	)
	require.NoError(t, err)
	return fset, f
}

type checkCase struct {
	name      string
	src       string
	wantCount int
	wantMsg   string
	wantFunc  string
}

// checkCases are the table-driven detection cases for checkFile.
var checkCases = []checkCase{
	{
		name: "bad Pipe1 handoff flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe1(s, handlerFn)
}
func handlerFn(s int) error { return nil }
`,
		wantCount: 1,
		wantMsg:   "F.Pipe1(seed, namedFn)",
		wantFunc:  "runThing",
	},
	{
		name: "valid Pipe3 not flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe3(s, step1, step2, step3)
}
func step1(int) int { return 0 }
func step2(int) int { return 0 }
func step3(int) error { return nil }
`,
		wantCount: 0,
	},
	{
		name: "non-entrypoint not flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func helper(s int) error {
	return F.Pipe1(s, handlerFn)
}
func handlerFn(int) error { return nil }
`,
		wantCount: 0,
	},
	{
		name: "inline func literal not flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe1(s, func(x int) error { return nil })
}
`,
		wantCount: 0,
	},
	{
		name: "combinator call not flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe1(s, F.Constant(0))
}
`,
		wantCount: 0,
	},
	{
		name: "alternate alias detected",
		src: `package testpkg
import fn "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return fn.Pipe1(s, handlerFn)
}
func handlerFn(int) error { return nil }
`,
		wantCount: 1,
	},
	{
		name: "default alias detected",
		src: `package testpkg
import "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return function.Pipe1(s, handlerFn)
}
func handlerFn(int) error { return nil }
`,
		wantCount: 1,
	},
	{
		name: "blank import ignored",
		src: `package testpkg
import _ "github.com/IBM/fp-go/v2/function"
func runThing(s int) error { return nil }
`,
		wantCount: 0,
	},
	{
		name: "function package not imported",
		src: `package testpkg
func runThing(s int) error { return nil }
`,
		wantCount: 0,
	},
	{
		name: "selector continuation flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe1(s, pkg.Handler)
}
`,
		wantCount: 1,
	},
	{
		name: "generic instantiation flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe1(s, Handler[int])
}
`,
		wantCount: 1,
	},
	{
		name: "Pipe1 with explicit type args flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe1[int, error](s, handlerFn)
}
func handlerFn(int) error { return nil }
`,
		wantCount: 1,
	},
	{
		name: "parenthesized continuation flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe1(s, (handlerFn))
}
func handlerFn(int) error { return nil }
`,
		wantCount: 1,
	},
	{
		name: "method-receiver entrypoint flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func (t T) runThing(s int) error {
	return F.Pipe1(s, handlerFn)
}
func handlerFn(int) error { return nil }
type T struct{}
`,
		wantCount: 1,
		wantFunc:  "runThing",
	},
	{
		name: "multi-value return flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) (int, error) {
	return 0, F.Pipe1(s, handlerFn)
}
func handlerFn(int) error { return nil }
`,
		wantCount: 1,
	},
	{
		name: "exemption with reason honored",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
// fp-go:allow-pipe1-handoff reused by runA and runB
func runThing(s int) error {
	return F.Pipe1(s, handlerFn)
}
func handlerFn(int) error { return nil }
`,
		wantCount: 0,
	},
	{
		name: "directive without reason flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
// fp-go:allow-pipe1-handoff
func runThing(s int) error {
	return F.Pipe1(s, handlerFn)
}
func handlerFn(int) error { return nil }
`,
		wantCount: 1,
		wantMsg:   "non-empty reason",
	},
	{
		name: "typoed directive not honored",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
// fp-go:allow-pipe1-handoffx reason
func runThing(s int) error {
	return F.Pipe1(s, handlerFn)
}
func handlerFn(int) error { return nil }
`,
		wantCount: 1,
		wantMsg:   "F.Pipe1(seed, namedFn)",
	},
	{
		name: "Pipe1 wrong arity not flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe1(s)
}
`,
		wantCount: 0,
	},
}

func TestCheckFileTable(t *testing.T) {
	cfg := withDefaults(Config{})
	for _, tc := range checkCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, tc.src)
			vs := checkFile(fset, f, cfg)
			require.Len(t, vs, tc.wantCount)
			if tc.wantMsg != "" {
				require.Contains(t, vs[0].Message, tc.wantMsg)
			}
			if tc.wantFunc != "" {
				require.Equal(t, tc.wantFunc, vs[0].Function)
			}
		})
	}
}

// seedCases are the table-driven detection cases for the opt-in
// applied-seed rule (Config.RequirePointFreeSeed = true).
var seedCases = []checkCase{
	{
		name: "applied seed flagged",
		src: `package testpkg
import (
	"context"
	F "github.com/IBM/fp-go/v2/function"
)
type State struct{ Ctx context.Context }
func seedState(ctx context.Context) State { return State{Ctx: ctx} }
func runThing(ctx context.Context) error {
	return F.Pipe2(seedState(ctx), step, fold)
}
func step(State) State { return State{} }
func fold(State) error { return nil }
`,
		wantCount: 1,
		wantMsg:   "applied call",
	},
	{
		name: "struct-literal seed not flagged",
		src: `package testpkg
import (
	"context"
	F "github.com/IBM/fp-go/v2/function"
)
type Input struct{ Ctx context.Context }
type State struct{ Ctx context.Context }
func seedState(in Input) State { return State{Ctx: in.Ctx} }
func runThing(ctx context.Context) error {
	return F.Pipe3(Input{Ctx: ctx}, seedState, step, fold)
}
func step(State) State { return State{} }
func fold(State) error { return nil }
`,
		wantCount: 0,
	},
	{
		name: "applied seed without param ref not flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func makeSeed() int { return 0 }
func runThing(s int) error {
	return F.Pipe2(makeSeed(), step, fold)
}
func step(int) int { return 0 }
func fold(int) error { return nil }
`,
		wantCount: 0,
	},
	{
		name: "applied seed with nested param ref flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
type State struct{ V string }
func seedState(s string) State { return State{V: s} }
func runThing(s string) error {
	return F.Pipe2(seedState(s + "x"), step, fold)
}
func step(State) State { return State{} }
func fold(State) error { return nil }
`,
		wantCount: 1,
	},
	{
		name: "applied seed alternate alias flagged",
		src: `package testpkg
import fn "github.com/IBM/fp-go/v2/function"
func makeSeed(s int) int { return s }
func runThing(s int) error {
	return fn.Pipe2(makeSeed(s), step, fold)
}
func step(int) int { return 0 }
func fold(int) error { return nil }
`,
		wantCount: 1,
	},
	{
		name: "applied seed generic PipeN flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func makeSeed(s int) int { return s }
func runThing(s int) error {
	return F.Pipe2[int, error](makeSeed(s), fold)
}
func fold(int) error { return nil }
`,
		wantCount: 1,
	},
	{
		name: "applied seed exemption with reason honored",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func makeSeed(s int) int { return s }
// fp-go:allow-applied-seed reused by runA and runB
func runThing(s int) error {
	return F.Pipe2(makeSeed(s), fold)
}
func fold(int) error { return nil }
`,
		wantCount: 0,
	},
	{
		name: "applied seed directive without reason flagged",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func makeSeed(s int) int { return s }
// fp-go:allow-applied-seed
func runThing(s int) error {
	return F.Pipe2(makeSeed(s), fold)
}
func fold(int) error { return nil }
`,
		wantCount: 1,
		wantMsg:   "non-empty reason",
	},
	{
		name: "applied seed typoed directive not honored",
		src: `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func makeSeed(s int) int { return s }
// fp-go:allow-applied-seedx reason
func runThing(s int) error {
	return F.Pipe2(makeSeed(s), fold)
}
func fold(int) error { return nil }
`,
		wantCount: 1,
		wantMsg:   "applied call",
	},
}

func TestAppliedSeedTable(t *testing.T) {
	cfg := withDefaults(Config{RequirePointFreeSeed: true})
	for _, tc := range seedCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, tc.src)
			vs := checkFile(fset, f, cfg)
			require.Len(t, vs, tc.wantCount)
			if tc.wantMsg != "" {
				require.Contains(t, vs[0].Message, tc.wantMsg)
			}
		})
	}
}

// seedCasesOff checks that with RequirePointFreeSeed = false (the
// default), applied-seed patterns are never flagged.
func TestAppliedSeedDisabledByDefault(t *testing.T) {
	cfg := withDefaults(Config{})
	for _, tc := range seedCases {
		if tc.name == "applied seed rule off by default" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, tc.src)
			require.Empty(t, checkFile(fset, f, cfg))
		})
	}
}

func TestCustomEntrypointPredicate(t *testing.T) {
	src := `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func handleThing(s int) error {
	return F.Pipe1(s, handlerFn)
}
func handlerFn(int) error { return nil }
`
	fset, f := parse(t, src)
	// default (run*) does not flag handleThing.
	require.Empty(
		t,
		checkFile(fset, f, withDefaults(Config{})),
	)
	// custom predicate does flag it.
	custom := withDefaults(Config{
		IsEntrypoint: func(name string) bool {
			return name == "handleThing"
		},
	})
	require.Len(t, checkFile(fset, f, custom), 1)
}

func TestCheckMalformedReturnsError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "bad.go"),
		[]byte("package testpkg\nfunc broken(\n"),
		0o600,
	))
	_, err := Check(Config{Roots: []string{dir}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse")
}

func TestCheckDedupesRoots(t *testing.T) {
	dir := writeFixture(t, `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe1(s, handlerFn)
}
func handlerFn(int) error { return nil }
`)
	vs, err := Check(Config{Roots: []string{dir, dir}})
	require.NoError(t, err)
	require.Len(
		t,
		vs,
		1,
		"duplicate roots must not double-count",
	)
}

func TestCheckSkipsTestdataDir(t *testing.T) {
	dir := t.TempDir()
	// A deliberately unparseable file under testdata/ must not abort
	// the run; testdata is pruned.
	td := filepath.Join(dir, "testdata")
	require.NoError(t, os.Mkdir(td, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(td, "broken.go"),
		[]byte("not even go"),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "clean.go"),
		[]byte(`package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe3(s, a, b, c)
}
func a(int) int { return 0 }
func b(int) int { return 0 }
func c(int) error { return nil }
`),
		0o600,
	))
	vs, err := Check(Config{Roots: []string{dir}})
	require.NoError(t, err)
	require.Empty(t, vs)
}

func TestRequirePassesWhenClean(t *testing.T) {
	dir := writeFixture(t, `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe3(s, step1, step2, step3)
}
func step1(int) int { return 0 }
func step2(int) int { return 0 }
func step3(int) error { return nil }
`)
	r := &fakeReporter{}
	Require(r, Config{Roots: []string{dir}})
	require.Empty(t, r.errors)
	require.Empty(t, r.fatal)
}

func TestRequireReportsViolations(t *testing.T) {
	dir := writeFixture(t, `package testpkg
import F "github.com/IBM/fp-go/v2/function"
func runThing(s int) error {
	return F.Pipe1(s, handlerFn)
}
func handlerFn(int) error { return nil }
`)
	r := &fakeReporter{}
	Require(r, Config{Roots: []string{dir}})
	require.Len(t, r.errors, 1)
	require.Contains(t, r.errors[0], "F.Pipe1(seed, namedFn)")
	require.Empty(t, r.fatal)
}

func TestRequireFatalfOnParseError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "bad.go"),
		[]byte("package testpkg\nfunc broken(\n"),
		0o600,
	))
	r := &fakeReporter{}
	Require(r, Config{Roots: []string{dir}})
	require.NotEmpty(t, r.fatal)
	require.Contains(t, r.fatal, "pipelinecheck:")
	require.Empty(t, r.errors)
}

// fakeReporter is a minimal Reporter for asserting Require's failure
// paths without the real testing.T.
type fakeReporter struct {
	errors []string
	fatal  string
}

func (f *fakeReporter) Helper() {}

func (f *fakeReporter) Errorf(
	format string,
	args ...any,
) {
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}

func (f *fakeReporter) Fatalf(
	format string,
	args ...any,
) {
	f.fatal = fmt.Sprintf(format, args...)
}

// writeFixture writes src to a single a.go file in a temp dir and
// returns the dir path.
func writeFixture(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "a.go"),
		[]byte(src),
		0o600,
	))
	return dir
}
