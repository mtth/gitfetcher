package gitfetcher

import (
	"io/fs"
	"log/slog"
	"os"
	"path"
	"slices"
)

type target struct {
	// path to gitdir (not working tree)
	path   string
	isBare bool
}

var (
	maxDepth       uint8 = 3
	ignoredFolders       = []string{"node_modules"}
	fileSystem     fs.FS = os.DirFS("/")
)

// findTargets walks the filesystem to find all repository targets under the root. Nested git
// directories are ignored. The root directory is never considered a valid repository.
func findTargets(root string) ([]target, error) {
	slog.Debug("Finding targets", dataAttrs(slog.String("root", root)))

	depths := make(map[string]uint8)
	var targets []target
	if err := fs.WalkDir(fileSystem, root, func(fp string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		slog.Debug("Walking directory entry.", dataAttrs(slog.String("path", fp), slog.String("name", entry.Name())))
		if !entry.IsDir() {
			return nil
		}
		depth := depths[path.Dir(fp)] + 1
		if depth > maxDepth || slices.Contains(ignoredFolders, entry.Name()) {
			return fs.SkipDir
		}
		if isGitDir(fp) {
			targets = append(targets, target{path: fp, isBare: entry.Name() != ".git"})
			return fs.SkipDir
		}
		depths[fp] = depth
		return nil
	}); err != nil {
		return nil, err
	}
	return targets, nil
}

// isGitDir returns whether path is a valid git directory. The logic is a simplified version of the
// flow in https://stackoverflow.com/a/65499840 and may lead to false positives.
func isGitDir(path string) bool {
	entries, err := fs.ReadDir(fileSystem, path)
	if err != nil {
		slog.Warn("Git directory read failed.", errAttr(err))
		return false
	}
	names := make(map[string]bool)
	for _, entry := range entries {
		names[entry.Name()] = true
	}
	return names["HEAD"] && names["objects"] && names["refs"]
}
