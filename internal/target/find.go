package target

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"slices"

	"github.com/mtth/gitfetcher/internal/fspath"
)

var (
	// maxDepth is the maximum filesystem depth explored when searching for targets in FindTargets.
	maxDepth uint8 = 4

	// ignoredFolders contains folder names which are ignored when searching for targets.
	ignoredFolders = []string{"node_modules"}

	// errTargetSearchFailed is returned when FindTargets failed.
	errTargetSearchFailed = errors.New("unable to find targets")
)

// FindTargets walks the filesystem to find all repository targets under the root. Nested git
// directories are ignored. The root directory is never considered a valid repository.
func Find(root fspath.Local) ([]Target, error) {
	slog.Debug("Finding targets", slog.String("root", root))

	// root, err := filepath.Abs(root)
	// except.Must(err == nil, "can't determine absolute root: %v", err)
	depths := make(map[fspath.Local]uint8)
	var targets []Target
	// TODO: fs.WalkDir behaves strangely with absolute roots. Investigate if there is a cleaner way
	// to implement this which also supports testing (ideally still via testfs).
	if err := fs.WalkDir(fileSystem, unabs(root), func(fpath fspath.POSIX, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		depth := depths[path.Dir(fpath)] + 1
		if depth > maxDepth || slices.Contains(ignoredFolders, entry.Name()) {
			return fs.SkipDir
		}
		tgt, err := FromPath("/" + fpath)
		if err != nil {
			return err
		}
		if tgt != nil {
			targets = append(targets, tgt)
			return fs.SkipDir
		}
		depths[fpath] = depth
		return nil
	}); err != nil {
		return nil, fmt.Errorf("%w: %v", errTargetSearchFailed, err)
	}

	slog.Info(fmt.Sprintf("Found %d targets.", len(targets)))
	return targets, nil
}
