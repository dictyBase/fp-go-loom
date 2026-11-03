package strutils

import (
	"strings"
	"testing"

	M "github.com/IBM/fp-go/v2/monoid"
	"github.com/stretchr/testify/require"
)

var separatorCases = []struct {
	name string
	m    M.Monoid[string]
	want string
}{
	{"Space", Space, "a b"},
	{"Comma", Comma, "a, b"},
	{"Semicolon", Semicolon, "a; b"},
	{"Colon", Colon, "a:b"},
	{"Label", Label, "a: b"},
	{"MiddleDot", MiddleDot, "a · b"},
	{"EmDash", EmDash, "a — b"},
	{"Newline", Newline, "a\nb"},
}

func TestSeparatorMonoids(t *testing.T) {
	for _, tc := range separatorCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.m.Concat("a", "b"))
		})
	}
}

// The monoid laws let callers regroup joins freely.
func TestSeparatorMonoidLaws(t *testing.T) {
	for _, tc := range separatorCases {
		t.Run(tc.name, func(t *testing.T) {
			e := tc.m.Empty()
			require.Equal(t, "a", tc.m.Concat(e, "a"))
			require.Equal(t, "a", tc.m.Concat("a", e))
			require.Equal(t,
				tc.m.Concat(tc.m.Concat("a", "b"), "c"),
				tc.m.Concat("a", tc.m.Concat("b", "c")),
			)
		})
	}
}

func TestEmptyOperandDropsSeparator(t *testing.T) {
	require.Equal(t, "a", MiddleDot.Concat("a", ""))
	require.Equal(t, "b", MiddleDot.Concat("", "b"))
	require.Equal(t, "", MiddleDot.Concat("", ""))
}

func TestJoins(t *testing.T) {
	parts := []string{"a", "", "b", "c"}
	require.Equal(t, "a b c", JoinSpace(parts))
	require.Equal(t, "a, b, c", JoinComma(parts))
	require.Equal(t, "a; b; c", JoinSemicolon(parts))
	require.Equal(t, "a · b · c", JoinMiddleDot(parts))
	require.Equal(t, "a\nb\nc", JoinLines(parts))
	require.Equal(t, "", JoinComma(nil))
	require.Equal(t, "", JoinComma([]string{"", ""}))
}

func TestCollapseWhitespace(t *testing.T) {
	require.Equal(
		t,
		"a b c",
		CollapseWhitespace("  a \t b\n\nc  "),
	)
	require.Equal(t, "", CollapseWhitespace(" \n "))
}

func TestWrappers(t *testing.T) {
	require.Equal(t, "(x)", Parens("x"))
	require.Equal(t, "[x]", Brackets("x"))
	require.Equal(t, `"x"`, Quote("x"))
	require.Equal(t, "x.", Period("x"))
	require.Equal(t, "<x>", Wrap("<", ">")("x"))
	require.Equal(t, "()", Parens(""))
}

func TestMapNonEmpty(t *testing.T) {
	upper := MapNonEmpty(strings.ToUpper)
	require.Equal(t, "ABC", upper("abc"))
	require.Equal(t, "", upper(""))

	tag := MapNonEmpty(Parens)
	require.Equal(t, "x (doi)", Space.Concat("x", tag("doi")))
	require.Equal(t, "x", Space.Concat("x", tag("")))
}

type author struct {
	last     string
	initials string
}

func lastName(a author) string { return a.last }

func initials(a author) string { return a.initials }

func TestConcatFields(t *testing.T) {
	name := ConcatFields[author](Space)(
		[]func(author) string{lastName, initials},
	)
	require.Equal(t, "Smith A", name(author{"Smith", "A"}))
	require.Equal(t, "Jones", name(author{last: "Jones"}))
	require.Equal(t, "", name(author{}))

	none := ConcatFields[author](Space)(nil)
	require.Equal(t, "", none(author{"Smith", "A"}))
}
