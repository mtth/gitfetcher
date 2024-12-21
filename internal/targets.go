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

	"github.com/mtth/gitfetcher/internal/except"
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
	// LastUpdatedAt is the most recent time at which a remote reference was updated. May be zero.
	RemoteLastUpdatedAt time.Time
}

// TargetFromPath creates a Target from a local path.
func TargetFromPath(p string) Target {
	var maxTime time.Time
	for _, remoteTime := range remoteRefUpdateTimes(p) {
		if remoteTime.After(maxTime) {
			maxTime = remoteTime
		}
	}
	return Target{Path: p, IsBare: path.Base(p) != ".git", RemoteLastUpdatedAt: maxTime}
}

func unabs(p string) string {
	return strings.TrimPrefix(p, "/")
}

// RemoteRefs returns information about the repository's remote git references from a gitdir path.
func remoteRefUpdateTimes(p string) map[string]time.Time {
	refs := make(map[string]time.Time)
	root := path.Join(p, "refs/remotes", remote)
	if err := fs.WalkDir(fileSystem, unabs(root), func(fp string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			if info, err := entry.Info(); err != nil {
				slog.Warn("Ref info error, skipping.", except.LogErrAttr(err))
			} else {
				// The path module doesn't have an equivalent to filepath.Rel unfortunately, so we use
				// string prefixes...
				rel, _ := strings.CutPrefix(fp, root+"/")
				refs[rel] = info.ModTime()
			}
		}
		return nil
	}); err != nil {
		slog.Warn("Failed to get remote refs.", except.LogErrAttr(err), dataAttrs(slog.String("path", p)))
	}
	return refs
}

var (
	// maxDepth is the maximum filesystem depth explored when searching for targets in FindTargets.
	maxDepth uint8 = 5

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
	// TODO: fs.WalkDir behaves strangely with absolute roots. Investigate if there is a cleaner way
	// to implement this which also supports testing (ideally still via testfs).
	if err := fs.WalkDir(fileSystem, unabs(root), func(fp string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		depth := depths[path.Dir(fp)] + 1
		if depth > maxDepth || slices.Contains(ignoredFolders, entry.Name()) {
			return fs.SkipDir
		}
		if ok, err := isGitDir(fp); ok {
			targets = append(targets, TargetFromPath("/"+fp))
			return fs.SkipDir
		} else if err != nil {
			slog.Warn("Git directory read failed.", except.LogErrAttr(err))
		}
		if ok, err := isGitDir(path.Join(fp, ".git")); ok {
			targets = append(targets, TargetFromPath("/"+fp+"/.git"))
			return fs.SkipDir
		} else if err != fs.ErrNotExist {
			slog.Warn("Git work directory read failed.", except.LogErrAttr(err))
		}
		depths[fp] = depth
		return nil
	}); err != nil {
		return nil, fmt.Errorf("%w: %v", errTargetSearchFailed, err)
	}

	slog.Info(fmt.Sprintf("Found %d targets.", len(targets)))
	return targets, nil
}

// isGitDir returns whether p is a valid git directory. The logic is a simplified version of the
// flow in https://stackoverflow.com/a/65499840 and may lead to false positives.
func isGitDir(p string) (bool, error) {
	entries, err := fs.ReadDir(fileSystem, unabs(p))
	if err != nil {
		return false, err
	}
	names := make(map[string]bool)
	for _, entry := range entries {
		names[entry.Name()] = true
	}
	return names["HEAD"] && names["objects"] && names["refs"], nil
}
