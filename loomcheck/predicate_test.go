package loomcheck

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

var predicateCases = []struct {
	name      string
	src       string
	wantCount int
	wantMsg   string
}{
	{
		name: "len greater than zero flagged",
		src: `package testpkg
var p = func(xs []int) bool { return len(xs) > 0 }`,
		wantCount: 1,
		wantMsg:   "predarrays.IsNonEmpty",
	},
	{
		name: "len not equal zero flagged",
		src: `package testpkg
var p = func(xs []int) bool { return len(xs) != 0 }`,
		wantCount: 1,
		wantMsg:   "predarrays.IsNonEmpty",
	},
	{
		name: "len at least n flagged",
		src: `package testpkg
var p = func(s string) bool { return len(s) >= 3 }`,
		wantCount: 1,
		wantMsg:   "MinLen",
	},
	{
		name: "len at most n flagged",
		src: `package testpkg
var p = func(s string) bool { return len(s) <= 5 }`,
		wantCount: 1,
		wantMsg:   "MaxLen",
	},
	{
		name: "len equal n flagged",
		src: `package testpkg
var p = func(s string) bool { return len(s) == 3 }`,
		wantCount: 1,
		wantMsg:   "LenEq",
	},
	{
		name: "equality with literal flagged",
		src: `package testpkg
var p = func(s string) bool { return s == "go" }`,
		wantCount: 1,
		wantMsg:   "predord.StrEq",
	},
	{
		name: "inequality with literal flagged",
		src: `package testpkg
var p = func(n int) bool { return n != 5 }`,
		wantCount: 1,
		wantMsg:   "NotEqual",
	},
	{
		name: "exclusive range flagged",
		src: `package testpkg
var p = func(n int) bool { return 1 <= n && n < 10 }`,
		wantCount: 1,
		wantMsg:   "predord.IntBetween(lo, hi)",
	},
	{
		name: "inclusive range flagged",
		src: `package testpkg
var p = func(n int) bool { return 1 <= n && n <= 10 }`,
		wantCount: 1,
		wantMsg:   "IntBetweenInclusive",
	},
	{
		name: "negation flagged",
		src: `package testpkg
type S struct{ NoCite bool }
var p = func(s S) bool { return !s.NoCite }`,
		wantCount: 1,
		wantMsg:   "P.Not",
	},
	{
		name: "strings.HasSuffix flagged",
		src: `package testpkg
import "strings"
var p = func(s string) bool {
	return strings.HasSuffix(s, ".go")
}`,
		wantCount: 1,
		wantMsg:   "predstrings.HasSuffix",
	},
	{
		name: "IsDir flagged",
		src: `package testpkg
import "io/fs"
var p = func(fi fs.FileInfo) bool { return fi.IsDir() }`,
		wantCount: 1,
		wantMsg:   "predfs.IsDirInfo",
	},
	{
		name: "IsRegular flagged",
		src: `package testpkg
import "io/fs"
var p = func(fi fs.FileInfo) bool {
	return fi.Mode().IsRegular()
}`,
		wantCount: 1,
		wantMsg:   "predfs.IsRegularInfo",
	},
	{
		name: "plain field access not flagged",
		src: `package testpkg
type S struct{ JSON bool }
var p = func(s S) bool { return s.JSON }`,
		wantCount: 0,
	},
	{
		name: "multi statement body not flagged",
		src: `package testpkg
var p = func(xs []int) bool {
	total := 0
	for _, x := range xs {
		total += x
	}
	return total > 0
}`,
		wantCount: 0,
	},
	{
		name: "non bool literal not flagged",
		src: `package testpkg
var p = func(xs []int) int { return len(xs) }`,
		wantCount: 0,
	},
	{
		name: "two param literal not flagged",
		src: `package testpkg
var p = func(a, b int) bool { return a == 1 }`,
		wantCount: 0,
	},
	{
		name: "comparison against non literal not flagged",
		src: `package testpkg
var p = func(a int) bool { return a == someVar }
var someVar int`,
		wantCount: 0,
	},
}

func TestHandRolledPredicates_Table(t *testing.T) {
	for _, tc := range predicateCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, tc.src)
			vs := checkHandRolledPredicates(
				fset,
				f,
				DefaultAllowDirective,
				false,
			)
			require.Len(t, vs, tc.wantCount)
			if tc.wantMsg != "" {
				require.Contains(t, vs[0].Message, tc.wantMsg)
			}
		})
	}
}

func TestHandRolledPredicates_AllowDirective(t *testing.T) {
	src := `package testpkg
// fp-go:allow-hand-rolled-predicate SDK fixes the callback shape
func adapter() func([]int) bool {
	return func(xs []int) bool { return len(xs) > 0 }
}`
	fset, f := parse(t, src)
	vs := checkHandRolledPredicates(
		fset,
		f,
		DefaultAllowDirective,
		false,
	)
	require.Empty(t, vs)
}

func TestHandRolledPredicates_DirectiveNeedsReason(
	t *testing.T,
) {
	src := `package testpkg
// fp-go:allow-hand-rolled-predicate
func adapter() func([]int) bool {
	return func(xs []int) bool { return len(xs) > 0 }
}`
	fset, f := parse(t, src)
	vs := checkHandRolledPredicates(
		fset,
		f,
		DefaultAllowDirective,
		false,
	)
	require.Len(t, vs, 1)
}

func TestHandRolledPredicates_CustomDirective(t *testing.T) {
	src := `package testpkg
// custom:allow-pred legacy shim
func adapter() func([]int) bool {
	return func(xs []int) bool { return len(xs) > 0 }
}`
	fset, f := parse(t, src)
	vs := checkHandRolledPredicates(
		fset,
		f,
		"custom:allow-pred",
		false,
	)
	require.Empty(t, vs)
}

func TestHandRolledPredicates_StrictMode(t *testing.T) {
	src := `package testpkg
type S struct{ JSON bool }
var p = func(s S) bool { return s.JSON }`
	fset, f := parse(t, src)

	lenient := checkHandRolledPredicates(
		fset,
		f,
		DefaultAllowDirective,
		false,
	)
	require.Empty(t, lenient)

	strict := checkHandRolledPredicates(
		fset,
		f,
		DefaultAllowDirective,
		true,
	)
	require.Len(t, strict, 1)
	require.Contains(t, strict[0].Message, "fp-go-loom catalog")
}

func TestHandRolledPredicates_ReportsEnclosingFunc(
	t *testing.T,
) {
	src := `package testpkg
func builder() func([]int) bool {
	return func(xs []int) bool { return len(xs) > 0 }
}`
	fset, f := parse(t, src)
	vs := checkHandRolledPredicates(
		fset,
		f,
		DefaultAllowDirective,
		false,
	)
	require.Len(t, vs, 1)
	require.Equal(t, "builder", vs[0].Function)
}

func TestCheck_ScansDirectory(t *testing.T) {
	dir := t.TempDir()
	good := "package testpkg\nvar A = 1\n"
	bad := "package testpkg\n" +
		"var P = func(xs []int) bool { return len(xs) > 0 }\n"
	writeFile(t, filepath.Join(dir, "good.go"), good)
	writeFile(t, filepath.Join(dir, "bad.go"), bad)

	vs, err := Check(Config{Roots: []string{dir}})
	require.NoError(t, err)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].String(), "bad.go")
}

func TestCheck_SkipsTestAndGeneratedFiles(t *testing.T) {
	dir := t.TempDir()
	bad := "var P = func(xs []int) bool { return len(xs) > 0 }\n"
	writeFile(
		t,
		filepath.Join(dir, "thing_test.go"),
		"package testpkg\n"+bad,
	)
	writeFile(
		t,
		filepath.Join(dir, "zz_gen.go"),
		"// Code generated by tool. DO NOT EDIT.\n"+
			"package testpkg\n"+bad,
	)

	vs, err := Check(Config{Roots: []string{dir}})
	require.NoError(t, err)
	require.Empty(t, vs)
}

func TestCheck_SkipsConfiguredDirs(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "generated")
	require.NoError(t, os.MkdirAll(sub, 0o750))
	writeFile(
		t,
		filepath.Join(sub, "bad.go"),
		"package testpkg\n"+
			"var P = func(xs []int) bool { return len(xs) > 0 }\n",
	)

	vs, err := Check(Config{
		Roots:    []string{dir},
		SkipDirs: []string{"generated"},
	})
	require.NoError(t, err)
	require.Empty(t, vs)
}

func TestRequire_ReportsViolations(t *testing.T) {
	dir := t.TempDir()
	writeFile(
		t,
		filepath.Join(dir, "bad.go"),
		"package testpkg\n"+
			"var P = func(xs []int) bool { return len(xs) > 0 }\n",
	)

	rec := &recorder{}
	Require(rec, Config{Roots: []string{dir}})
	require.Len(t, rec.errors, 1)
	require.Contains(t, rec.errors[0], "predarrays.IsNonEmpty")
}

func TestRequire_SilentWhenClean(t *testing.T) {
	dir := t.TempDir()
	writeFile(
		t,
		filepath.Join(dir, "good.go"),
		"package testpkg\nvar A = 1\n",
	)

	rec := &recorder{}
	Require(rec, Config{Roots: []string{dir}})
	require.Empty(t, rec.errors)
	require.Empty(t, rec.fatals)
}

func TestViolation_String(t *testing.T) {
	v := Violation{
		Position: token.Position{Filename: "a.go", Line: 3},
		Function: "runThing",
		Message:  "msg",
	}
	require.Contains(t, v.String(), "a.go:3")
	require.Contains(t, v.String(), "runThing")
	require.Contains(t, v.String(), "msg")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(
		t,
		os.WriteFile(path, []byte(content), 0o600),
	)
}

// recorder captures Reporter calls for assertions.
type recorder struct {
	errors []string
	fatals []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, sprintf(format, args...))
}

// sprintf is a local alias so the recorder avoids importing fmt in
// two places.
func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
