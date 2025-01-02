package except

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMust(t *testing.T) {
	t.Run("no-op", func(t *testing.T) {
		Must(true, "ok")
	})

	t.Run("panic", func(t *testing.T) {
		require.Panics(t, func() {
			Must(false, "panic")
		})
	})
}
