package arrutils_test

import (
	"strconv"
	"testing"

	O "github.com/IBM/fp-go/v2/option"
	"github.com/stretchr/testify/require"

	arrutils "github.com/dictyBase/fp-go-loom/array"
)

func TestCompact(t *testing.T) {
	t.Run("extracts Some values", func(t *testing.T) {
		opts := []O.Option[string]{
			O.Some("a"),
			O.None[string](),
			O.Some("b"),
		}
		require.Equal(
			t,
			[]string{"a", "b"},
			arrutils.Compact[string](opts),
		)
	})
	t.Run("all None returns empty", func(t *testing.T) {
		require.Empty(
			t,
			arrutils.Compact(
				[]O.Option[int]{O.None[int](), O.None[int]()},
			),
		)
	})
	t.Run("nil input returns empty", func(t *testing.T) {
		require.Empty(t, arrutils.Compact([]O.Option[int](nil)))
	})
}

func TestParseWith(t *testing.T) {
	parseInts := arrutils.ParseWith(strconv.Atoi)

	t.Run(
		"parses valid and discards invalid",
		func(t *testing.T) {
			require.Equal(
				t,
				[]int{1, 42, 7},
				parseInts(
					[]string{"1", "bad", "42", "nope", "7"},
				),
			)
		},
	)
	t.Run("all invalid returns empty", func(t *testing.T) {
		require.Empty(t, parseInts([]string{"x", "y"}))
	})
	t.Run("empty input returns empty", func(t *testing.T) {
		require.Empty(t, parseInts([]string{}))
	})

	parseFloats := arrutils.ParseWith(
		func(s string) (float64, error) {
			return strconv.ParseFloat(s, 64)
		},
	)
	t.Run("parses floats", func(t *testing.T) {
		require.Equal(
			t,
			[]float64{1.5, 3.14},
			parseFloats([]string{"1.5", "bad", "3.14"}),
		)
	})
}
