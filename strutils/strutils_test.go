package strutils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJoinStrings(t *testing.T) {
	t.Run("joins two strings", func(t *testing.T) {
		result := JoinStrings("hello", "world")
		require.Equal(t, "helloworld", result)
	})

	t.Run("joins three strings", func(t *testing.T) {
		result := JoinStrings("a", "b", "c")
		require.Equal(t, "abc", result)
	})

	t.Run("empty returns empty", func(t *testing.T) {
		result := JoinStrings()
		require.Equal(t, "", result)
	})

	t.Run("single string", func(t *testing.T) {
		result := JoinStrings("solo")
		require.Equal(t, "solo", result)
	})
}
