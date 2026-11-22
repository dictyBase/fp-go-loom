package pipelinecheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	wantBoolExprMsg = "hand-written boolean expression"

	boolExprPreamble = `package testpkg
import (
	F "github.com/IBM/fp-go/v2/function"
	predord "github.com/dictyBase/fp-go-loom/predicate/ord"
)
type loc struct{ Scheme, Kind string }
func scheme(l loc) string { return l.Scheme }
`
)

var boolExprCases = []struct {
	name      string
	body      string
	wantCount int
}{
	{
		name:      "equality flagged",
		body:      `func isS3(s string) bool { return s == "s3" }`,
		wantCount: 1,
	},
	{
		name:      "logical or flagged",
		body:      `func isAuto(s string) bool { return s == "" || s == "auto" }`,
		wantCount: 1,
	},
	{
		name:      "negation flagged",
		body:      `func notDir(dir bool) bool { return !dir }`,
		wantCount: 1,
	},
	{
		name: "locals before comparison flagged",
		body: `func fits(l loc) bool {
	n := len(scheme(l))
	return n > 2
}`,
		wantCount: 1,
	},
	{
		name:      "predicate pipeline clean",
		body:      `func isS3(l loc) bool { return F.Pipe2(l, scheme, predord.StrEq("s3")) }`,
		wantCount: 0,
	},
	{
		name:      "predicate var clean",
		body:      `var isS3 = F.Flow2(scheme, predord.StrEq("s3"))`,
		wantCount: 0,
	},
	{
		name:      "two parameters clean",
		body:      `func same(a, b loc) bool { return a.Kind == b.Kind }`,
		wantCount: 0,
	},
	{
		name:      "method clean",
		body:      `func (l loc) local() bool { return l.Scheme == "file" }`,
		wantCount: 0,
	},
	{
		name:      "non-bool result clean",
		body:      `func size(l loc) int { return len(l.Kind) + 1 }`,
		wantCount: 0,
	},
}

func TestBoolExpr_Table(t *testing.T) {
	for _, tc := range boolExprCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, f := parse(t, boolExprPreamble+tc.body)
			vs := checkNoBoolExpr(
				fset,
				f,
				DefaultAllowBoolExprDirective,
			)
			require.Len(t, vs, tc.wantCount)
			if tc.wantCount > 0 {
				require.Contains(
					t,
					vs[0].Message,
					wantBoolExprMsg,
				)
			}
		})
	}
}

func TestBoolExpr_FileWithoutFpGoIgnored(t *testing.T) {
	fset, f := parse(t, `package testpkg
func isS3(s string) bool { return s == "s3" }`)
	require.Empty(
		t,
		checkNoBoolExpr(fset, f, DefaultAllowBoolExprDirective),
	)
}

func TestBoolExpr_AllowDirective(t *testing.T) {
	fset, f := parse(t, boolExprPreamble+`
// fp-go:allow-bool-expr bit mask has no predicate form
func circular(b byte) bool { return b&1 != 0 }`)
	require.Empty(
		t,
		checkNoBoolExpr(fset, f, DefaultAllowBoolExprDirective),
	)
}
