package pipelinecheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	wantDirectMsg = "combinator applied directly"

	directPreamble = `package testpkg
import (
	"strings"
	A "github.com/IBM/fp-go/v2/array"
	E "github.com/IBM/fp-go/v2/either"
	F "github.com/IBM/fp-go/v2/function"
	O "github.com/IBM/fp-go/v2/option"
	predord "github.com/dictyBase/fp-go-loom/predicate/ord"
)
func plain(s string) string { return s }
`
)

var directCases = []struct {
	name      string
	body      string
	wantCount int
}{
	{
		name: "combinator applied directly flagged",
		body: `func bad(xs []string) []string {
	return A.Map(plain)(xs)
}`,
		wantCount: 1,
	},
	{
		name: "generic combinator applied directly flagged",
		body: `func bad(e E.Either[error, string]) string {
	return E.GetOrElse[error, string](
		func(error) string { return "" })(e)
}`,
		wantCount: 1,
	},
	{
		name: "parenthesized direct apply flagged",
		body: `func bad(xs []string) []string {
	return (A.Map(plain))(xs)
}`,
		wantCount: 1,
	},
	{
		name: "loom combinator applied directly flagged",
		body: `func bad(s string) bool {
	return predord.MinStrLen(2)(s)
}`,
		wantCount: 1,
	},
	{
		name: "curried fp-go call reports the inner apply once",
		body: `func bad(a, b string) bool {
	return F.Curry2(strings.HasPrefix)(a)(b)
}`,
		wantCount: 1,
	},
	{
		name: "Pipe with combinator argument clean",
		body: `func good(xs []string) []string {
	return F.Pipe1(xs, A.Map(plain))
}`,
		wantCount: 0,
	},
	{
		name: "partial application as argument clean",
		body: `func good() func(O.Option[string]) string {
	return O.GetOrElse(F.Constant("x"))
}`,
		wantCount: 0,
	},
	{
		name: "plain call as first argument clean",
		body: `func good(s string) O.Option[string] {
	return O.Some(plain(s))
}`,
		wantCount: 0,
	},
	{
		name: "non-fp-go curried call clean",
		body: `func curry(a string) func(string) string {
	return func(b string) string { return a + b }
}
func good() string { return curry("a")("b") }`,
		wantCount: 0,
	},
	{
		name: "function value from variable clean",
		body: `var mapper = A.Map(plain)
func good(xs []string) []string { return mapper(xs) }`,
		wantCount: 0,
	},
}

func TestDirectApply_Table(t *testing.T) {
	for _, tc := range directCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, directPreamble+tc.body)
			vs := checkNoDirectApply(
				fset, f, DefaultAllowDirectApplyDirective)
			require.Len(t, vs, tc.wantCount)
			if tc.wantCount > 0 {
				require.Contains(t, vs[0].Message, wantDirectMsg)
			}
		})
	}
}

func TestDirectApply_FileWithoutFpGoIgnored(t *testing.T) {
	fset, f := parse(t, `package testpkg
func curry(a string) func(string) string {
	return func(b string) string { return a + b }
}
func f() string { return curry("a")("b") }`)
	require.Empty(t, checkNoDirectApply(
		fset, f, DefaultAllowDirectApplyDirective))
}

func TestDirectApply_AllowDirective(t *testing.T) {
	fset, f := parse(t, directPreamble+`
// fp-go:allow-direct-apply the library returns a function
func ok(xs []string) []string {
	return A.Map(plain)(xs)
}`)
	require.Empty(t, checkNoDirectApply(
		fset, f, DefaultAllowDirectApplyDirective))
}

func TestDirectApply_DirectiveWithoutReason(t *testing.T) {
	fset, f := parse(t, directPreamble+`
// fp-go:allow-direct-apply
func ok(xs []string) []string {
	return A.Map(plain)(xs)
}`)
	vs := checkNoDirectApply(
		fset, f, DefaultAllowDirectApplyDirective)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, "non-empty reason")
}

func TestDirectApply_CheckAndRequireOverRoots(t *testing.T) {
	dir := writeFixture(t, directPreamble+`
func bad(xs []string) []string { return A.Map(plain)(xs) }`)
	cfg := Config{Roots: []string{dir}}

	vs, err := CheckNoDirectApply(cfg)
	require.NoError(t, err)
	require.Len(t, vs, 1)

	rec := &fakeReporter{}
	RequireNoDirectApply(rec, cfg)
	require.Len(t, rec.errors, 1)
	require.Contains(t, rec.errors[0], wantDirectMsg)
}

func TestDirectApply_ScanErrorIsFatal(t *testing.T) {
	rec := &fakeReporter{}
	RequireNoDirectApply(rec, Config{Roots: []string{
		t.TempDir() + "/missing",
	}})
	require.Contains(t, rec.fatal, "pipelinecheck:")
}
