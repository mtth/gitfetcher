//go:build test

package fspath

import (
	"path/filepath"
	"runtime"
)

// ProjectRoot returns a path to the git repo's root folder.
func ProjectRoot() Local {
	_, fpath, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(fpath), "..", "..")
}
