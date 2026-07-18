package predarrays_test

import (
	"testing"

	predarrays "github.com/dictyBase/fp-go-loom/predicate/array"
	"github.com/stretchr/testify/require"
)

func TestIsNonEmpty(t *testing.T) {
	pred := predarrays.IsNonEmpty[int]()
	require.False(t, pred(nil))
	require.False(t, pred([]int{}))
	require.True(t, pred([]int{1}))
	require.True(t, pred([]int{1, 2, 3}))
}

func TestMinLen(t *testing.T) {
	pred := predarrays.MinLen[string](3)
	require.False(t, pred([]string{"a", "b"}))
	require.True(t, pred([]string{"a", "b", "c"}))
	require.True(t, pred([]string{"a", "b", "c", "d"}))
}

func TestMaxLen(t *testing.T) {
	pred := predarrays.MaxLen[string](3)
	require.True(t, pred([]string{"a", "b", "c"}))
	require.False(t, pred([]string{"a", "b", "c", "d"}))
	require.True(t, pred(nil))
}

func TestLenEq(t *testing.T) {
	pred := predarrays.LenEq[int](2)
	require.False(t, pred([]int{1}))
	require.True(t, pred([]int{1, 2}))
	require.False(t, pred([]int{1, 2, 3}))
}
