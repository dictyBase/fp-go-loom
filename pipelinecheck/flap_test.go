package pipelinecheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const flapPreamble = `package testpkg
import (
	A "github.com/IBM/fp-go/v2/array"
	En "github.com/IBM/fp-go/v2/endomorphism"
	F "github.com/IBM/fp-go/v2/function"
	I "github.com/IBM/fp-go/v2/identity"
	O "github.com/IBM/fp-go/v2/option"
)
type T struct{ A int }
`

var flapCases = []struct {
	name      string
	body      string
	wantCount int
}{
	{
		name:      "array Flap flagged",
		body:      `func bad(fs []func(int) int) []int { return F.Pipe1(fs, A.Flap[int](1)) }`,
		wantCount: 1,
	},
	{
		name:      "identity Flap flagged",
		body:      `func bad(f func(int) int) int { return F.Pipe1(f, I.Flap[int](1)) }`,
		wantCount: 1,
	},
	{
		name:      "option MonadFlap flagged",
		body:      `func bad(f O.Option[func(int) int]) O.Option[int] { return O.MonadFlap(f, 1) }`,
		wantCount: 1,
	},
	{
		name:      "F.Flip flagged",
		body:      `var bad = F.Flip(F.Curry2(func(a, b int) int { return a - b }))`,
		wantCount: 1,
	},
	{
		name:      "Flip around Flap counts twice",
		body:      `var bad = F.Flip(A.Flap[int, int])`,
		wantCount: 2,
	},
	{
		name: "Flap feeding En.Reduce allowed",
		body: `func build(edits []En.Endomorphism[T]) T {
	return F.Pipe2(edits, A.Flap[En.Endomorphism[T]](T{}), En.Reduce[T])
}`,
	},
	{
		name: "plain call clean",
		body: `func good(f func(int) int) int { return f(1) }`,
	},
}

func TestFlap_Table(t *testing.T) {
	for _, tc := range flapCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, flapPreamble+tc.body)
			vs := checkNoFlap(fset, f, DefaultAllowFlapDirective)
			require.Len(t, vs, tc.wantCount)
			for _, v := range vs {
				require.Contains(t, v.Message, "point-free Flap")
			}
		})
	}
}

func TestFlap_NoFpGoImportIgnored(t *testing.T) {
	fset, f := parse(t, `package testpkg
type X struct{}
func (X) Flap(int) int { return 0 }
func use(x X) int { return x.Flap(1) }`)
	require.Empty(
		t,
		checkNoFlap(fset, f, DefaultAllowFlapDirective),
	)
}

func TestFlap_AllowDirective(t *testing.T) {
	fset, f := parse(t, flapPreamble+`
// fp-go:allow-flap applies a fixed list of builders to one spec
func ok(fs []func(int) int) []int { return F.Pipe1(fs, A.Flap[int](1)) }`)
	require.Empty(
		t,
		checkNoFlap(fset, f, DefaultAllowFlapDirective),
	)
}

func TestFlap_DirectiveWithoutReason(t *testing.T) {
	fset, f := parse(t, flapPreamble+`
// fp-go:allow-flap
func ok(fs []func(int) int) []int { return F.Pipe1(fs, A.Flap[int](1)) }`)
	vs := checkNoFlap(fset, f, DefaultAllowFlapDirective)
	require.Len(t, vs, 1)
	require.Contains(
		t,
		vs[0].Message,
		"requires a non-empty reason",
	)
}

func TestFlap_CheckAndRequireOverRoots(t *testing.T) {
	vs, err := CheckNoFlap(Config{Roots: []string{"."}})
	require.NoError(t, err)
	require.Empty(t, vs)
}
