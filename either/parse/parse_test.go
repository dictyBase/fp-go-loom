package eitherparse_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	E "github.com/IBM/fp-go/v2/either"

	eitherparse "github.com/dictyBase/fp-go-loom/either/parse"
)

func TestParseInt_success(t *testing.T) {
	result := eitherparse.ParseInt("42")
	require.True(t, E.IsRight(result))
	require.Equal(
		t,
		42,
		E.GetOrElse(func(error) int { return 0 })(result),
	)
}

func TestParseInt_invalidString(t *testing.T) {
	result := eitherparse.ParseInt("abc")
	require.True(t, E.IsLeft(result))
	msg := E.Fold(
		func(err error) string { return err.Error() },
		func(int) string { return "" },
	)(result)
	require.Contains(t, msg, "failed to parse int")
	require.Contains(t, msg, "abc")
}

func TestParseInt_emptyString(t *testing.T) {
	result := eitherparse.ParseInt("")
	require.True(t, E.IsLeft(result))
	msg := E.Fold(
		func(err error) string { return err.Error() },
		func(int) string { return "" },
	)(result)
	require.Contains(t, msg, "failed to parse int")
}
