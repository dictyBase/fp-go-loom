package pipelinecheck

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	wantLoopMsg   = "imperative loop"
	wantBranchMsg = "imperative branch"
)

var loopCases = []struct {
	name      string
	src       string
	wantCount int
}{
	{
		name: "range flagged",
		src: `package testpkg
func sum(xs []int) int {
	t := 0
	for _, x := range xs { t += x }
	return t
}`,
		wantCount: 1,
	},
	{
		name: "three-clause for flagged",
		src: `package testpkg
func count() int {
	n := 0
	for i := 0; i < 3; i++ { n++ }
	return n
}`,
		wantCount: 1,
	},
	{
		name: "nested loops each flagged",
		src: `package testpkg
func grid(xs [][]int) int {
	n := 0
	for _, r := range xs { for range r { n++ } }
	return n
}`,
		wantCount: 2,
	},
	{
		name: "loop in func literal at package scope flagged",
		src: `package testpkg
var f = func(xs []int) int {
	n := 0
	for range xs { n++ }
	return n
}`,
		wantCount: 1,
	},
	{
		name: "loop inside TryCatchError callback allowed",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
func read(xs []int) IOE.IOEither[error, int] {
	return IOE.TryCatchError(func() (int, error) {
		n := 0
		for range xs { n++ }
		return n, nil
	})
}`,
		wantCount: 0,
	},
	{
		name: "loop in non-fp-go TryCatchError callback flagged",
		src: `package testpkg
import IOE "example.com/other/ioeither"
func read(xs []int) int {
	return IOE.TryCatchError(func() int {
		n := 0
		for range xs { n++ }
		return n
	})
}`,
		wantCount: 1,
	},
	{
		name: "no loop clean",
		src: `package testpkg
func id(v int) int { return v }`,
		wantCount: 0,
	},
}

func TestImperativeLoop_Table(t *testing.T) {
	for _, tc := range loopCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, tc.src)
			vs := checkControlFlow(fset, f, isLoopStmt,
				loopMessage, DefaultAllowLoopDirective)
			require.Len(t, vs, tc.wantCount)
			if tc.wantCount > 0 {
				require.Contains(t, vs[0].Message, wantLoopMsg)
			}
		})
	}
}

func TestImperativeLoop_AllowDirective(t *testing.T) {
	src := `package testpkg
// fp-go:allow-loop AppendChild is the only API of the node tree
func build(xs []int) int {
	n := 0
	for range xs { n++ }
	return n
}`
	fset, f := parse(t, src)
	vs := checkControlFlow(fset, f, isLoopStmt,
		loopMessage, DefaultAllowLoopDirective)
	require.Empty(t, vs)
}

func TestImperativeLoop_DirectiveWithoutReason(t *testing.T) {
	src := `package testpkg
// fp-go:allow-loop
func build(xs []int) int {
	n := 0
	for range xs { n++ }
	return n
}`
	fset, f := parse(t, src)
	vs := checkControlFlow(fset, f, isLoopStmt,
		loopMessage, DefaultAllowLoopDirective)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, "non-empty reason")
}

func TestImperativeLoop_DirectiveIsPerFunction(t *testing.T) {
	src := `package testpkg
// fp-go:allow-loop mutable tree adapter
func a(xs []int) { for range xs {} }
func b(xs []int) { for range xs {} }`
	fset, f := parse(t, src)
	vs := checkControlFlow(fset, f, isLoopStmt,
		loopMessage, DefaultAllowLoopDirective)
	require.Len(t, vs, 1)
	require.Equal(t, "b", vs[0].Function)
}

var branchCases = []struct {
	name      string
	src       string
	wantCount int
}{
	{
		name: "if in named function flagged",
		src: `package testpkg
func pick(v int) int {
	if v > 0 { return v }
	return 0
}`,
		wantCount: 1,
	},
	{
		name: "switch flagged",
		src: `package testpkg
func name(v int) string {
	switch v {
	case 1: return "one"
	}
	return ""
}`,
		wantCount: 1,
	},
	{
		name: "type switch flagged",
		src: `package testpkg
func kind(v any) string {
	switch v.(type) {
	case int: return "int"
	}
	return ""
}`,
		wantCount: 1,
	},
	{
		name: "else-if chain flagged once per if",
		src: `package testpkg
func pick(v int) int {
	if v > 1 { return 2 } else if v > 0 { return 1 }
	return 0
}`,
		wantCount: 2,
	},
	{
		name: "if err in TryCatchError callback allowed",
		src: `package testpkg
import IOE "github.com/IBM/fp-go/v2/ioeither"
func read() IOE.IOEither[error, int] {
	return IOE.TryCatchError(func() (int, error) {
		v, err := get()
		if err != nil { return 0, err }
		return v, nil
	})
}
func get() (int, error) { return 0, nil }`,
		wantCount: 0,
	},
	{
		name: "O.TryCatch callback allowed",
		src: `package testpkg
import O "github.com/IBM/fp-go/v2/option"
func read() O.Option[int] {
	return O.TryCatch(func() (int, error) {
		if true { return 1, nil }
		return 0, nil
	})
}`,
		wantCount: 0,
	},
	{
		name: "no branch clean",
		src: `package testpkg
func id(v int) int { return v }`,
		wantCount: 0,
	},
}

func TestImperativeBranch_Table(t *testing.T) {
	for _, tc := range branchCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, tc.src)
			vs := checkControlFlow(fset, f, isBranchStmt,
				branchMessage, DefaultAllowBranchDirective)
			require.Len(t, vs, tc.wantCount)
			if tc.wantCount > 0 {
				require.Contains(t, vs[0].Message, wantBranchMsg)
			}
		})
	}
}

func TestImperativeBranch_AllowDirective(t *testing.T) {
	src := `package testpkg
// fp-go:allow-branch the SDK returns a nilable pointer
func pick(v *int) int {
	if v == nil { return 0 }
	return *v
}`
	fset, f := parse(t, src)
	vs := checkControlFlow(fset, f, isBranchStmt,
		branchMessage, DefaultAllowBranchDirective)
	require.Empty(t, vs)
}

func TestControlFlow_CheckAndRequireOverRoots(t *testing.T) {
	dir := writeFixture(t, `package testpkg
func f(xs []int) int {
	n := 0
	for range xs { if n > 0 { n++ } }
	return n
}`)
	cfg := Config{Roots: []string{dir}}

	loops, err := CheckNoImperativeLoop(cfg)
	require.NoError(t, err)
	require.Len(t, loops, 1)

	branches, err := CheckNoImperativeBranch(cfg)
	require.NoError(t, err)
	require.Len(t, branches, 1)

	rec := &fakeReporter{}
	RequireNoImperativeLoop(rec, cfg)
	require.Len(t, rec.errors, 1)
	rec = &fakeReporter{}
	RequireNoImperativeBranch(rec, cfg)
	require.Len(t, rec.errors, 1)
}

func TestControlFlow_ScanErrorIsFatal(t *testing.T) {
	cfg := Config{Roots: []string{filepath.Join(
		t.TempDir(), "missing")}}
	rec := &fakeReporter{}
	RequireNoImperativeLoop(rec, cfg)
	require.Contains(t, rec.fatal, "pipelinecheck:")
}

func TestFpGoAliases(t *testing.T) {
	_, f := parse(t, `package testpkg
import (
	"strings"
	A "github.com/IBM/fp-go/v2/array"
	"github.com/IBM/fp-go/v2/option"
	_ "github.com/IBM/fp-go/v2/either"
	other "example.com/x/function"
	predord "github.com/dictyBase/fp-go-loom/predicate/ord"
)`)
	require.Equal(t,
		map[string]bool{
			"A": true, "option": true, "predord": true,
		},
		fpGoAliases(f))
}
