package target

import (
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mtth/gitfetcher/internal/effect"
	"github.com/mtth/gitfetcher/internal/fspath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromPath(t *testing.T) {
	root := fspath.ProjectRoot()

	t.Run("non-git dir", func(t *testing.T) {
		got, err := FromPath(filepath.Join(root, "internal"))
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("missing dir", func(t *testing.T) {
		_, err := FromPath("missing")
		require.ErrorContains(t, err, "no such file")
	})

	t.Run("valid dir", func(t *testing.T) {
		got, err := FromPath(root)
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, filepath.Join(root, GitDirName), got.GitDir())
		assert.Equal(t, root, got.WorkDir())
		assert.False(t, IsBare(got))
	})
}

func TestTarget_RemoteLastUpdatedAt(t *testing.T) {
	t1 := time.UnixMilli(time.Hour.Milliseconds())
	t2 := time.UnixMilli(2 * time.Hour.Milliseconds())
	t3 := time.UnixMilli(3 * time.Hour.Milliseconds())

	defer swapFileSystem(fstest.MapFS{
		"root/one.git/HEAD":                        emptyFile,
		"root/one.git/objects":                     emptyFile,
		"root/one.git/refs/remotes/origin/main":    &fstest.MapFile{ModTime: t1},
		"root/one.git/refs/remotes/origin/foo/one": &fstest.MapFile{ModTime: t2},
		"root/one.git/refs/remotes/origin/foo/two": &fstest.MapFile{ModTime: t1},
		"root/one.git/refs/remotes/other/bar":      &fstest.MapFile{ModTime: t3},
	})()

	t.Run("underlying times ", func(t *testing.T) {
		times := remoteRefUpdateTimes("/root/one.git")
		assert.Equal(t, map[string]time.Time{"main": t1, "foo/one": t2, "foo/two": t1}, times)
	})

	t.Run("method", func(t *testing.T) {
		tgt, err := FromPath("/root/one.git")
		require.NoError(t, err)
		assert.Equal(t, t2, tgt.RemoteLastUpdatedAt())
	})
}

var emptyFile = &fstest.MapFile{}

func swapFileSystem(mapfs fstest.MapFS) func() {
	return effect.Swap[fs.FS](&fileSystem, mapfs)
}
