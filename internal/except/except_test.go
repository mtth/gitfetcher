package except

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestLogErrAttr(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		got := LogErrAttr(nil)
		assert.Equal(t, "[]", got.Value.String())
	})

	t.Run("no-nil error", func(t *testing.T) {
		got := LogErrAttr(errors.New("boom"))
		assert.Contains(t, got.Value.String(), "boom")
	})
}
