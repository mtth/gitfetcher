package target

import (
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/mtth/gitfetcher/internal/fspath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bare(dpath fspath.Local) Target {
	return realTarget{gitDir: dpath}
}

func nonBare(dpath fspath.Local) Target {
	return realTarget{gitDir: filepath.Join(dpath, GitDirName), workDir: dpath}
}

func TestFind(t *testing.T) {
	t.Run("invalid folder", func(t *testing.T) {
		defer swapFileSystem(fstest.MapFS{
			"foo": emptyFile,
		})()
		targets, err := Find("/bar")
		require.ErrorIs(t, err, errTargetSearchFailed)
		assert.Empty(t, targets)
	})

	t.Run("single repo", func(t *testing.T) {
		defer swapFileSystem(fstest.MapFS{
			"root/first/.git/HEAD":    emptyFile,
			"root/first/.git/objects": emptyFile,
			"root/first/.git/refs":    emptyFile,
			"root/other":              emptyFile,
		})()
		targets, err := Find("/root")
		require.NoError(t, err)
		assert.Equal(t, []Target{nonBare("/root/first")}, targets)
	})

	t.Run("ignores folders", func(t *testing.T) {
		defer swapFileSystem(fstest.MapFS{
			"root/node_modules/.git/HEAD":    emptyFile,
			"root/node_modules/.git/objects": emptyFile,
			"root/node_modules/.git/refs":    emptyFile,
		})()
		targets, err := Find("/root")
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
		targets, err := Find("/root")
		require.NoError(t, err)
		assert.Equal(t, []Target{nonBare("/root/parent")}, targets)
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
		targets, err := Find("/root")
		require.NoError(t, err)
		assert.Equal(t, []Target{bare("/root/one.git"), bare("/root/two.git")}, targets)
	})
}
