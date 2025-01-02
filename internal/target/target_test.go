package target

import (
	"fmt"
	"testing"

	"github.com/mtth/gitfetcher/internal/fspath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromPath(t *testing.T) {
	t.Run("non-git dir", func(t *testing.T) {
		got, err := FromPath(".")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("missing dir", func(t *testing.T) {
		_, err := FromPath("missing")
		require.ErrorContains(t, err, "no such file")
	})

	t.Run("valid dir", func(t *testing.T) {
		got, err := FromPath(fspath.ProjectRoot())
		require.NoError(t, err)
		assert.NotNil(t, got)
		fmt.Println(got)
		assert.False(t, got.RemoteLastUpdatedAt().IsZero())
	})
}
