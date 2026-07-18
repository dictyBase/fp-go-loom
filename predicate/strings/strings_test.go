package predstrings_test

import (
	"testing"
	"unicode"

	predstrings "github.com/dictyBase/fp-go-loom/predicate/strings"
	"github.com/stretchr/testify/require"
)

func TestLastIndexOf(t *testing.T) {
	require.Equal(
		t,
		7,
		predstrings.LastIndexOf("@")("foo@bar@baz"),
		"last @ at index 7",
	)
}

func TestHasSuffix(t *testing.T) {
	require.True(
		t,
		predstrings.HasSuffix("go")("hello.go"),
		"should have suffix .go",
	)
	require.False(
		t,
		predstrings.HasSuffix("go")("hello.py"),
		"should not have suffix .go",
	)
}

func TestContainsRuneClass(t *testing.T) {
	require.True(
		t,
		predstrings.ContainsRuneClass(unicode.IsUpper)("Hello"),
		"Hello has uppercase rune",
	)
	require.False(
		t,
		predstrings.ContainsRuneClass(unicode.IsUpper)("hello"),
		"hello has no uppercase rune",
	)
}

func TestHasAtSign(t *testing.T) {
	require.True(
		t,
		predstrings.HasAtSign("user@example.com"),
		"user@example.com contains @",
	)
	require.False(
		t,
		predstrings.HasAtSign("userexample.com"),
		"userexample.com has no @",
	)
}
