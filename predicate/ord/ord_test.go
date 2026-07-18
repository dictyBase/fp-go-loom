package predord_test

import (
	"testing"

	predord "github.com/dictyBase/fp-go-loom/predicate/ord"
	"github.com/stretchr/testify/require"
)

func TestMinStrLen(t *testing.T) {
	require.False(
		t,
		predord.MinStrLen(3)("hi"),
		"len 2 should be < 3",
	)
	require.True(
		t,
		predord.MinStrLen(3)("hey"),
		"len 3 should be >= 3",
	)
}

func TestIntBetween(t *testing.T) {
	require.True(
		t,
		predord.IntBetween(1, 10)(5),
		"5 is between 1 and 10",
	)
	require.False(
		t,
		predord.IntBetween(1, 10)(10),
		"10 is not < 10 (exclusive upper bound)",
	)
}

func TestMaxStrLen(t *testing.T) {
	require.True(
		t,
		predord.MaxStrLen(5)("hello"),
		"len 5 should be <= 5",
	)
	require.False(
		t,
		predord.MaxStrLen(5)("toolong"),
		"len 7 should be > 5",
	)
}

func TestStrLenEq(t *testing.T) {
	require.True(
		t,
		predord.StrLenEq(3)("hey"),
		"len 3 should equal 3",
	)
	require.False(
		t,
		predord.StrLenEq(3)("hi"),
		"len 2 should not equal 3",
	)
}

func TestNotEqualF64(t *testing.T) {
	require.False(
		t,
		predord.NotEqualF64(1.0)(1.0),
		"1.0 == 1.0 so NotEqual should be false",
	)
	require.True(
		t,
		predord.NotEqualF64(1.0)(2.0),
		"2.0 != 1.0 so NotEqual should be true",
	)
}
