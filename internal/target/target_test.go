package target

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromPath(t *testing.T) {
	t.Run("empty dir", func(t *testing.T) {
		got, err := FromPath(".")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("missing dir", func(t *testing.T) {
		_, err := FromPath("missing")
		require.ErrorContains(t, err, "no such file")
	})
}
