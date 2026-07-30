package pipelinecheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const wantFoldMsg = "returns nil"

// --- RedundantFold ---

var foldCases = []struct {
	name      string
	src       string
	wantCount int
	wantMsg   string
}{
	{
		name: "anonymous func discard flagged",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
type A struct{}
func wrapErr(e error) error { return e }
func bad(e E.Either[error, A]) error {
	return E.Fold(wrapErr, func(A) error { return nil })(e)
}`,
		wantCount: 1,
		wantMsg:   wantFoldMsg,
	},
	{
		name: "anonymous func discard left identity flagged",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
import F "github.com/IBM/fp-go/v2/function"
type A struct{}
func bad(e E.Either[error, A]) error {
	return E.Fold(F.Identity[error], func(a A) error { return nil })(e)
}`,
		wantCount: 1,
		wantMsg:   wantFoldMsg,
	},
	{
		name: "Constant1 nil conversion flagged",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
import F "github.com/IBM/fp-go/v2/function"
type A struct{}
func wrapErr(e error) error { return e }
func bad(e E.Either[error, A]) error {
	return E.Fold(wrapErr, F.Constant1[A](error(nil)))(e)
}`,
		wantCount: 1,
		wantMsg:   wantFoldMsg,
	},
	{
		name: "Constant1 bare nil flagged",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
import F "github.com/IBM/fp-go/v2/function"
type A struct{}
func wrapErr(e error) error { return e }
func bad(e E.Either[error, A]) error {
	return E.Fold(wrapErr, F.Constant1[A](nil))(e)
}`,
		wantCount: 1,
		wantMsg:   wantFoldMsg,
	},
	{
		name: "Match alias flagged",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
type A struct{}
func wrapErr(e error) error { return e }
func bad(e E.Either[error, A]) error {
	return E.Match(wrapErr, func(a A) error { return nil })(e)
}`,
		wantCount: 1,
		wantMsg:   wantFoldMsg,
	},
	{
		name: "curried application flagged once",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
import F "github.com/IBM/fp-go/v2/function"
type A struct{}
func bad(e E.Either[error, A]) error {
	return E.Fold(F.Identity[error], func(a A) error { return nil })(e)
}`,
		wantCount: 1,
		wantMsg:   wantFoldMsg,
	},
	{
		name: "both arms named functions not flagged",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
type A struct{}
func wrapErr(e error) error { return e }
func use(a A) error { return nil }
func good(e E.Either[error, A]) error {
	return E.Fold(wrapErr, use)(e)
}`,
		wantCount: 0,
	},
	{
		name: "right arm returns real value not flagged",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
type A struct{}
var realErr = error(nil)
func wrapErr(e error) error { return e }
func good(e E.Either[error, A]) error {
	return E.Fold(wrapErr, func(a A) error { return realErr })(e)
}`,
		wantCount: 0,
	},
	{
		name: "Constant1 real value not flagged",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
import F "github.com/IBM/fp-go/v2/function"
type A struct{}
var sentinel error
func wrapErr(e error) error { return e }
func good(e E.Either[error, A]) error {
	return E.Fold(wrapErr, F.Constant1[A](sentinel))(e)
}`,
		wantCount: 0,
	},
	{
		name: "multi-statement right arm not flagged",
		src: `package testpkg
import E "github.com/IBM/fp-go/v2/either"
type A struct{}
func wrapErr(e error) error { return e }
func good(e E.Either[error, A]) error {
	return E.Fold(wrapErr, func(a A) error { _ = a; return nil })(e)
}`,
		wantCount: 0,
	},
	{
		name: "user-type Fold not flagged",
		src: `package testpkg
type A struct{}
func (a A) Fold(l, r func(A) A) A { return A{} }
func bad() A {
	a := A{}
	return a.Fold(func(x A) A { return x }, func(x A) A { return A{} })
}`,
		wantCount: 0,
	},
}

func TestRedundantFold_Table(t *testing.T) {
	for _, c := range foldCases {
		t.Run(c.name, func(t *testing.T) {
			fset, f := parse(t, c.src)
			vs := checkRedundantFold(
				fset, f, eitherAliases(f),
				DefaultAllowRedundantFoldDirective,
			)
			require.Len(t, vs, c.wantCount)
			if c.wantCount > 0 {
				require.Contains(t, vs[0].Message, c.wantMsg)
			}
		})
	}
}

func TestRedundantFold_AllowDirective(t *testing.T) {
	src := `package testpkg
import E "github.com/IBM/fp-go/v2/either"
import F "github.com/IBM/fp-go/v2/function"
type A struct{}

// fp-go:allow-redundant-fold legacy fold preserves the old shape
func bad(e E.Either[error, A]) error {
	return E.Fold(F.Identity[error], func(a A) error { return nil })(e)
}`
	fset, f := parse(t, src)
	vs := checkRedundantFold(
		fset, f, eitherAliases(f),
		DefaultAllowRedundantFoldDirective,
	)
	require.Empty(t, vs)
}

func TestRedundantFold_DirectiveWithoutReason(t *testing.T) {
	src := `package testpkg
import E "github.com/IBM/fp-go/v2/either"
import F "github.com/IBM/fp-go/v2/function"
type A struct{}

// fp-go:allow-redundant-fold
func bad(e E.Either[error, A]) error {
	return E.Fold(F.Identity[error], func(a A) error { return nil })(e)
}`
	fset, f := parse(t, src)
	vs := checkRedundantFold(
		fset, f, eitherAliases(f),
		DefaultAllowRedundantFoldDirective,
	)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, "non-empty reason")
}

// --- Package-scope (no enclosing func) ---

func TestRedundantFold_PackageScope(t *testing.T) {
	src := `package testpkg
import E "github.com/IBM/fp-go/v2/either"
import F "github.com/IBM/fp-go/v2/function"
type A struct{}
var _ = E.Fold(F.Identity[error], func(a A) error { return nil })`
	fset, f := parse(t, src)
	vs := checkRedundantFold(
		fset, f, eitherAliases(f),
		DefaultAllowRedundantFoldDirective,
	)
	require.Len(t, vs, 1)
	require.Equal(t, "<package>", vs[0].Function)
}

// --- Public API ---

func TestRedundantFold_PublicAPI(t *testing.T) {
	cfg := Config{Roots: []string{"testdata/bad_fold"}}
	vs, err := CheckRedundantFold(cfg)
	require.NoError(t, err)
	require.Len(t, vs, 1)
	require.Contains(t, vs[0].Message, wantFoldMsg)

	// Check (entrypoint rules) does not run the Fold rule.
	ev, err := Check(cfg)
	require.NoError(t, err)
	require.Empty(t, ev)
}

func TestFixtures_GoodSilentForFold(t *testing.T) {
	cfg := Config{Roots: []string{"testdata/good"}}
	vs, err := CheckRedundantFold(cfg)
	require.NoError(t, err)
	require.Empty(t, vs)
}
