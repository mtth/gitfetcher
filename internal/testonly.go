//go:build test

package gitfetcher

// Swap temporarily replaces a variable with another. Call the returned function to restore the
// original value.
func swap[V any](ref *V, val V) func() {
	old := *ref
	*ref = val
	return func() { *ref = old }
}
