package eithererr_test

import (
	"testing"

	"github.com/dictyBase/fp-go-loom/eithererr"
	"github.com/stretchr/testify/require"
)

func TestConstErr_returnsErrorWithGivenMessage(t *testing.T) {
	fn := eithererr.ConstErr[string]("some error")
	err := fn("anything")
	require.EqualError(t, err, "some error")
}

func TestConstErr_ignoresArgument(t *testing.T) {
	fn := eithererr.ConstErr[int]("some error")
	require.EqualError(t, fn(0), "some error")
	require.EqualError(t, fn(42), "some error")
	require.EqualError(t, fn(-1), "some error")
}

func TestConstErr_returnsSameErrorInstance(t *testing.T) {
	fn := eithererr.ConstErr[string]("same error")
	require.ErrorIs(t, fn("a"), fn("b"))
}
