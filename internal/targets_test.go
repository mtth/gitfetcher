package gitfetcher

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	emptyFile = &fstest.MapFile{Data: []byte{}}
	folder    = &fstest.MapFile{Mode: fs.ModeDir}
)

func TestFindTargets(t *testing.T) {
	t.Run("single repo", func(t *testing.T) {
		defer swap[fs.FS](&fileSystem, fstest.MapFS{
			"root/first/.git/HEAD":    emptyFile,
			"root/first/.git/objects": emptyFile,
			"root/first/.git/refs":    emptyFile,
			"root/other":              emptyFile,
		})()
		targets, err := findTargets("root")
		require.NoError(t, err)
		assert.Equal(t, []target{{path: "root/first/.git"}}, targets)
	})

	t.Run("ignores folders", func(t *testing.T) {
		defer swap[fs.FS](&fileSystem, fstest.MapFS{
			"root/node_modules/.git/HEAD":    emptyFile,
			"root/node_modules/.git/objects": emptyFile,
			"root/node_modules/.git/refs":    emptyFile,
		})()
		targets, err := findTargets("root")
		require.NoError(t, err)
		assert.Empty(t, targets)
	})

	t.Run("ignores nested repositories", func(t *testing.T) {
		defer swap[fs.FS](&fileSystem, fstest.MapFS{
			"root/parent/.git/HEAD":                 emptyFile,
			"root/parent/.git/objects":              emptyFile,
			"root/parent/.git/refs":                 emptyFile,
			"root/parent/vendor/child/.git/HEAD":    emptyFile,
			"root/parent/vendor/child/.git/objects": emptyFile,
			"root/parent/vendor/child/.git/refs":    emptyFile,
		})()
		targets, err := findTargets("root")
		require.NoError(t, err)
		assert.Equal(t, []target{{path: "root/parent/.git"}}, targets)
	})

	t.Run("multiple bare repositories", func(t *testing.T) {
		defer swap[fs.FS](&fileSystem, fstest.MapFS{
			"root/one.git/HEAD":    emptyFile,
			"root/one.git/objects": emptyFile,
			"root/one.git/refs":    emptyFile,
			"root/two.git/HEAD":    emptyFile,
			"root/two.git/objects": emptyFile,
			"root/two.git/refs":    emptyFile,
		})()
		targets, err := findTargets("root")
		require.NoError(t, err)
		assert.Equal(t, []target{{path: "root/one.git", isBare: true}, {path: "root/two.git", isBare: true}}, targets)
	})
}
