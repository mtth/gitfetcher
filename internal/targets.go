package gitfetcher

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"slices"
	"strings"
	"time"
)

// remote is the name of the remote used for sources.
const remote = "origin"

// fileSystem is swapped out for testing.
var fileSystem = os.DirFS("/")

// Target is a local copy of a repository (mirror target).
type Target struct {
	// Path to local gitdir (not working tree).
	Path string
	// IsBare is true iff the repository does not have a working directory.
	IsBare bool
}

// RemoteRefs returns information about the repository's remote git references.
func (t *Target) RemoteRefs() map[string]time.Time {
	refs := make(map[string]time.Time)
	root := path.Join(t.Path, "refs/remotes", remote)
	if err := fs.WalkDir(fileSystem, root, func(fp string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			if info, err := entry.Info(); err != nil {
				slog.Warn("Ref info error, skipping.", errAttr(err))
			} else {
				// The path module doesn't have an equivalent to filepath.Rel unfortunately, so we use
				// string prefixes...
				rel, _ := strings.CutPrefix(fp, root+"/")
				refs[rel] = info.ModTime()
			}
		}
		return nil
	}); err != nil {
		slog.Warn("Failed to get remote refs.", errAttr(err), dataAttrs(slog.String("path", t.Path)))
	}
	return refs
}

var (
	// maxDepth is the maximum filesystem depth explored when searching for targets in FindTargets.
	maxDepth uint8 = 3

	// ignoredFolders contains folder names which are ignored when searching for targets.
	ignoredFolders = []string{"node_modules"}

	// errTargetSearchFailed is returned when FindTargets failed.
	errTargetSearchFailed = errors.New("unable to find targets")
)

// FindTargets walks the filesystem to find all repository targets under the root. Nested git
// directories are ignored. The root directory is never considered a valid repository.
func FindTargets(root string) ([]Target, error) {
	slog.Debug("Finding targets", dataAttrs(slog.String("root", root)))

	depths := make(map[string]uint8)
	var targets []Target
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
			targets = append(targets, Target{Path: fp, IsBare: entry.Name() != ".git"})
			return fs.SkipDir
		}
		depths[fp] = depth
		return nil
	}); err != nil {
		return nil, fmt.Errorf("%w: %v", errTargetSearchFailed, err)
	}

	slog.Info(fmt.Sprintf("Found %d targets.", len(targets)))
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
