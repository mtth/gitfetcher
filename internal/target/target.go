package target

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mtth/gitfetcher/internal/except"
	"github.com/mtth/gitfetcher/internal/fspath"
)

const (
	// DefaultRemote is the name of the remote used for sources.
	DefaultRemote = "origin"

	// GitDirName is the name of the gitdir folder inside a repo's workdir.
	GitDirName = ".git"
)

// Target is a local copy of a repository (mirror target).
type Target interface {
	// Path to local gitdir (not working tree).
	GitDir() fspath.Local
	// WorkDir is the rep's working directory, or empty if the repo is bare.
	WorkDir() fspath.Local
	// LastUpdatedAt is the most recent time at which a remote reference was updated. May be zero.
	RemoteLastUpdatedAt() time.Time
}

// IsBare returns true iff the input target does not have a work directory.
func IsBare(tgt Target) bool {
	return tgt.WorkDir() == ""
}

// realTarget is a filesystem-backed Target implementation.
type realTarget struct {
	gitDir, workDir fspath.Local
}

// GitDir implements Target.
func (t realTarget) GitDir() fspath.Local { return t.gitDir }

// WorkDir implements Target.
func (t realTarget) WorkDir() fspath.Local { return t.workDir }

// LastUpdatedAt implements Target.
func (t realTarget) RemoteLastUpdatedAt() time.Time {
	var maxTime time.Time
	for _, remoteTime := range remoteRefUpdateTimes(t.gitDir) {
		if remoteTime.After(maxTime) {
			maxTime = remoteTime
		}
	}
	return maxTime
}

func unabs(fpath fspath.Local) fspath.POSIX {
	absPath, err := filepath.Abs(fpath)
	except.Must(err == nil, "can't make path %v absolute: %v", fpath, err)
	return filepath.ToSlash(strings.TrimPrefix(absPath, string(filepath.Separator)))
}

// remoteRefUpdateTimes returns information about the repository's remote git references from a
// gitdir path.
func remoteRefUpdateTimes(fpath fspath.Local) map[string]time.Time {
	refs := make(map[fspath.Local]time.Time)
	root := filepath.Join(fpath, "refs", "remotes", DefaultRemote)
	if err := fs.WalkDir(fileSystem, unabs(root), func(fpath fspath.POSIX, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			if info, err := entry.Info(); err != nil {
				slog.Warn("Ref info error, skipping.", except.LogErrAttr(err))
			} else {
				rel, err := filepath.Rel(root, filepath.FromSlash("/"+fpath))
				if err != nil {
					return err
				}
				refs[rel] = info.ModTime()
			}
		}
		return nil
	}); err != nil {
		slog.Warn("Failed to get remote refs.", except.LogErrAttr(err), slog.String("path", fpath))
	}
	return refs
}

// FromPath returns a Target from a local path if it contains a repository. Otherwise it returns a
// nil target.
func FromPath(dpath fspath.Local) (Target, error) {
	var gitdir, workdir fspath.Local
	ok, err := isGitDir(dpath)
	if err != nil {
		return nil, err
	}
	if ok {
		gitdir = dpath
	} else {
		workdir = dpath
		gitdir = filepath.Join(dpath, GitDirName)
		ok, err := isGitDir(gitdir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
			return nil, err
		}
		if !ok {
			return nil, nil
		}
	}
	return realTarget{gitDir: gitdir, workDir: workdir}, nil
}

// fileSystem is swapped out for testing.
var fileSystem = os.DirFS("/")

// isGitDir returns whether path is a valid git directory. The logic is a simplified version of the
// flow in https://stackoverflow.com/a/65499840 and may lead to false positives.
func isGitDir(dpath fspath.Local) (bool, error) {
	entries, err := fs.ReadDir(fileSystem, unabs(dpath))
	if err != nil {
		return false, err
	}
	names := make(map[string]bool)
	for _, entry := range entries {
		names[entry.Name()] = true
	}
	return names["HEAD"] && names["objects"] && names["refs"], nil
}
