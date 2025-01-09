//go:build test

package effect

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Swap temporarily replaces a variable with another. Call the returned function to restore the
// original value.
func Swap[V any](ref *V, val V) func() {
	old := *ref
	*ref = val
	return func() { *ref = old }
}

// RequireEvar returns an environment variable, aborting if it is not set.
func RequireEvar(t *testing.T, name string) string {
	t.Helper()
	val, ok := os.LookupEnv(name)
	require.True(t, ok, "missing %s evar", name)
	return val
}
