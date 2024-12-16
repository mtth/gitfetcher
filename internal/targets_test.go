package gitfetcher

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var emptyFile = &fstest.MapFile{}

func TestFindTargets(t *testing.T) {
	t.Run("single repo", func(t *testing.T) {
		defer swapFileSystem(fstest.MapFS{
			"root/first/.git/HEAD":    emptyFile,
			"root/first/.git/objects": emptyFile,
			"root/first/.git/refs":    emptyFile,
			"root/other":              emptyFile,
		})()
		targets, err := FindTargets("/root")
		require.NoError(t, err)
		assert.Equal(t, []Target{{Path: "/root/first/.git"}}, targets)
	})

	t.Run("ignores folders", func(t *testing.T) {
		defer swapFileSystem(fstest.MapFS{
			"root/node_modules/.git/HEAD":    emptyFile,
			"root/node_modules/.git/objects": emptyFile,
			"root/node_modules/.git/refs":    emptyFile,
		})()
		targets, err := FindTargets("/root")
		require.NoError(t, err)
		assert.Empty(t, targets)
	})

	t.Run("ignores nested repositories", func(t *testing.T) {
		defer swapFileSystem(fstest.MapFS{
			"root/parent/.git/HEAD":                 emptyFile,
			"root/parent/.git/objects":              emptyFile,
			"root/parent/.git/refs":                 emptyFile,
			"root/parent/vendor/child/.git/HEAD":    emptyFile,
			"root/parent/vendor/child/.git/objects": emptyFile,
			"root/parent/vendor/child/.git/refs":    emptyFile,
		})()
		targets, err := FindTargets("/root")
		require.NoError(t, err)
		assert.Equal(t, []Target{{Path: "/root/parent/.git"}}, targets)
	})

	t.Run("multiple bare repositories", func(t *testing.T) {
		defer swapFileSystem(fstest.MapFS{
			"root/one.git/HEAD":    emptyFile,
			"root/one.git/objects": emptyFile,
			"root/one.git/refs":    emptyFile,
			"root/two.git/HEAD":    emptyFile,
			"root/two.git/objects": emptyFile,
			"root/two.git/refs":    emptyFile,
		})()
		targets, err := FindTargets("/root")
		require.NoError(t, err)
		assert.Equal(t, []Target{
			{Path: "/root/one.git", IsBare: true},
			{Path: "/root/two.git", IsBare: true},
		}, targets)
	})
}

func TestRemoteRefUpdateTimes(t *testing.T) {
	t1 := time.UnixMilli(time.Hour.Milliseconds())
	t2 := time.UnixMilli(2 * time.Hour.Milliseconds())
	t3 := time.UnixMilli(3 * time.Hour.Milliseconds())
	t.Run("simple", func(t *testing.T) {
		defer swapFileSystem(fstest.MapFS{
			"root/one.git/HEAD":                        emptyFile,
			"root/one.git/objects":                     emptyFile,
			"root/one.git/refs/remotes/origin/main":    &fstest.MapFile{ModTime: t1},
			"root/one.git/refs/remotes/origin/foo/one": &fstest.MapFile{ModTime: t2},
			"root/one.git/refs/remotes/origin/foo/two": &fstest.MapFile{ModTime: t3},
			"root/one.git/refs/remotes/other/bar":      &fstest.MapFile{ModTime: t2},
		})()
		times := remoteRefUpdateTimes("root/one.git")
		assert.Equal(t, map[string]time.Time{"main": t1, "foo/one": t2, "foo/two": t3}, times)
	})
}

func swapFileSystem(mapfs fstest.MapFS) func() {
	return swap[fs.FS](&fileSystem, mapfs)
}
