package gitfetcher

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path"
	"slices"

	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"github.com/mtth/gitfetcher/internal/fspath"
	"github.com/mtth/gitfetcher/internal/source"
	"github.com/mtth/gitfetcher/internal/target"
)

var errDuplicateSource = errors.New("duplicate source path")

var (
	FindTargets = target.Find
	LoadSources = source.Load
)

// Syncable contains all the information needed to mirror a repository.
type Syncable struct {
	// Absolute local path to the repository's gitdir.
	GitDir fspath.Local
	// Local target, if any.
	target *target.Target
	// Mirror source, if any. Present if target is nil.
	source *source.Source
	// True iff the repository should be created bare.
	bareInit bool
}

// GatherSyncables reconciles targets and sources into Syncable instances.
func GatherSyncables(
	targets []target.Target,
	sources []source.Source,
	root string,
	initLayout configpb.Options_Layout,
) ([]Syncable, error) {
	slog.Debug("Gathering syncables...")

	// We first index all sources by target path.
	sourcesByPath := make(map[string]*source.Source)
	for _, source := range sources {
		fp := source.RelPath
		if fp == "" {
			fp = source.FullName
			if initLayout == configpb.Options_BARE_LAYOUT {
				fp += ".git"
			} else {
				fp = path.Join(fp, ".git")
			}
		}
		fp = path.Join(root, fp)
		if _, ok := sourcesByPath[fp]; ok {
			return nil, fmt.Errorf("%w (%s)", errDuplicateSource, fp)
		}
		sourcesByPath[fp] = &source
	}

	// Then we iterate over targets to create syncables, adding a source if available.
	syncablesByPath := make(map[string]Syncable)
	for _, target := range targets {
		gitDir := target.GitDir()
		syncable := Syncable{GitDir: gitDir, target: &target}
		if source, ok := sourcesByPath[gitDir]; ok {
			syncable.source = source
		}
		syncablesByPath[gitDir] = syncable
	}
	// Finally, we look for sources which do not yet have a target.
	bareInit := initLayout == configpb.Options_BARE_LAYOUT
	for fp, source := range sourcesByPath {
		if _, ok := syncablesByPath[fp]; !ok {
			syncablesByPath[fp] = Syncable{GitDir: fp, source: source, bareInit: bareInit}
		}
	}

	slog.Info(fmt.Sprintf("Gathered %v syncables.", len(syncablesByPath)))
	syncables := slices.Collect(maps.Values(syncablesByPath))
	slices.SortFunc(syncables, func(s1, s2 Syncable) int { return cmp.Compare(s1.GitDir, s2.GitDir) })
	return syncables, nil
}

// SyncStatus returns the current SyncStatus of the syncable.
func (s *Syncable) SyncStatus() SyncStatus {
	switch {
	case s.target == nil:
		return SyncStatusMissing
	case s.source == nil || s.source.LastUpdatedAt.IsZero():
		return SyncStatusUnknown
	case (*s.target).RemoteLastUpdatedAt().Before(s.source.LastUpdatedAt):
		return SyncStatusStale
	default:
		return SyncStatusFresh
	}
}

// SyncStatus captures possible states of the local repository vs its remote.
type SyncStatus int

//go:generate go run github.com/dmarkham/enumer -type=SyncStatus -trimprefix SyncStatus -transform snake-upper
const (
	// Not enough information.
	SyncStatusUnknown SyncStatus = iota
	// No local copy of the repository.
	SyncStatusMissing
	// A local copy of the repository exists but is not up-to-date.
	SyncStatusStale
	// The local copy of the repository exists and is up-to-date.
	SyncStatusFresh
)

var errSyncFailed = errors.New("sync failed")

func checkSyncStep(err error) {
	if err != nil {
		panic(fmt.Errorf("%w: %v", errSyncFailed, err))
	}
}

// Sync syncs local copies in the root folder of each source. Missing local repositories will be
// created, others will be updated as needed.
func (s *Syncable) Sync(ctx context.Context) (err error) {
	slog.Debug(fmt.Sprintf("Syncing %+v...", s))

	defer func() {
		if r := recover(); r != nil {
			if rerr, ok := r.(error); ok && errors.Is(rerr, errSyncFailed) {
				err = errors.Join(err, rerr)
				return
			}
			panic(r)
		}
	}()

	status := s.SyncStatus()
	if status == SyncStatusMissing {
		s.createTarget(ctx)
	}
	s.updateMetadata(ctx)
	if status != SyncStatusFresh {
		s.updateContents(ctx)
	}
	slog.Info(fmt.Sprintf("Synced %+v.", s), slog.String("status", status.String()))
	return
}

func (s *Syncable) createTarget(ctx context.Context) {
	checkSyncStep(os.MkdirAll(s.GitDir, 0755))

	// We don't use git clone to avoid having the credentials saved in the repo's config and share
	// more logic with the update function below.
	initArgs := []string{"init"}
	if branch := s.source.DefaultBranch; branch != "" {
		initArgs = append(initArgs, "-b", branch)
	}
	if s.bareInit {
		initArgs = append(initArgs, "--bare")
	}
	runGitCommand(ctx, s.GitDir, initArgs)

	// TODO: Confirm that we do not need -m to specify a branch when adding the remote.
	runGitCommand(ctx, s.GitDir, []string{"remote", "add", target.DefaultRemote, s.source.FetchURL})

	slog.Debug("Created target.")
}

func (s *Syncable) defaultRemoteRef() string {
	if source := s.source; source != nil && source.DefaultBranch != "" {
		return fmt.Sprintf("refs/remotes/%s/%s", target.DefaultRemote, source.DefaultBranch)
	}
	return ""
}

func (s *Syncable) isBare() bool {
	if tgt := s.target; tgt != nil {
		return target.IsBare(*tgt)
	}
	return s.bareInit
}

func (s *Syncable) gitPath(lp string) string {
	return path.Join(s.GitDir, lp)
}

// WorkDir returns the syncable's working directory, or an empty string if absent (for bare repos).
func (s *Syncable) WorkDir() string {
	if s.isBare() {
		return ""
	}
	return path.Dir(s.GitDir)
}

// RootDir returns the syncable's outermost directory (workdir if present, else gitdir).
func (s *Syncable) RootDir() string {
	return cmp.Or(s.WorkDir(), s.GitDir)
}

func (s *Syncable) updateContents(ctx context.Context) {
	slog.Debug("Updating contents...")

	fetchFlags := []string{"fetch", "--all"}
	if src := s.source; src != nil {
		fetchFlags = append(fetchFlags, src.FetchFlags...)
	}
	runGitCommand(ctx, s.GitDir, fetchFlags)

	if s.isBare() {
		// Update HEAD directly so that gitweb shows the most recent remote commit.
		if ref := s.defaultRemoteRef(); ref != "" {
			runGitCommand(ctx, s.GitDir, []string{"update-ref", "refs/heads/HEAD", ref})
		}
	} else {
		if !fileExists(s.gitPath("refs/heads/HEAD")) {
			// No working directory yet.
			if source := s.source; source != nil && source.DefaultBranch != "" {
				runGitCommand(ctx, s.GitDir, []string{"checkout", source.DefaultBranch})
			}
		} else {
			runGitCommand(ctx, s.GitDir, []string{"merge", "--ff-only"})
		}
	}
	slog.Debug("Updated contents.")
}

func (s *Syncable) updateMetadata(ctx context.Context) {
	if source := s.source; source != nil {
		runGitCommand(ctx, s.GitDir, []string{"config", "set", "gitweb.url", source.FetchURL})

		// This allows the remote branches to show up in the summary page's HEADS section.
		runGitCommand(ctx, s.GitDir, []string{"config", "set", "gitweb.extraBranchRefs", "remotes"})

		if desc := source.Description; desc != "" {
			checkSyncStep(os.WriteFile(s.gitPath("description"), []byte(desc), 0644))
		}
	}
	slog.Debug("Updated metadata.")
}

var (
	runGitCommand = func(ctx context.Context, cwd string, args []string) {
		runCommand(ctx, cwd, "git", args)
	}
)

func fileExists(fp string) bool {
	_, err := os.Stat(fp)
	return !errors.Is(err, fs.ErrNotExist)
}

// runCommand executes a command, panicking if it fails.
func runCommand(ctx context.Context, cwd, name string, args []string) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	stderr, err := cmd.StderrPipe()
	checkSyncStep(err)
	checkSyncStep(cmd.Start())
	errData, _ := io.ReadAll(stderr)
	if err := cmd.Wait(); err != nil {
		checkSyncStep(fmt.Errorf("%w: %v", err, string(errData)))
	}
}
