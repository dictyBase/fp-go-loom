package predbytes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasPositiveLen(t *testing.T) {
	require.False(t, HasPositiveLen(nil))
	require.False(t, HasPositiveLen([]byte{}))
	require.True(t, HasPositiveLen([]byte{0}))
	require.True(t, HasPositiveLen([]byte("hello")))
}

func TestIsNonEmpty(t *testing.T) {
	require.False(t, IsNonEmpty(nil))
	require.False(t, IsNonEmpty([]byte{}))
	require.True(t, IsNonEmpty([]byte{0}))
	require.True(t, IsNonEmpty([]byte("data")))
}
