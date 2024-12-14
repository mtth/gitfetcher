package gitfetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
)

// GetSyncStatus returns the current SyncStatus of a source.
func GetSyncStatus(src *Source, opts *configpb.Options) SyncStatus {
	syncable := newSyncable(src, opts)
	lastSyncedAt := syncableModTime(syncable)
	if lastSyncedAt.IsZero() {
		return SyncStatusAbsent
	}
	if src.LastUpdatedAt.IsZero() || lastSyncedAt.Before(src.LastUpdatedAt) {
		return SyncStatusStale
	}
	return SyncStatusFresh
}

// Sync syncs local copies in the root folder of each source. Missing local repositories will be
// created, others will be updated as needed.
func Sync(ctx context.Context, srcs []*Source, opts *configpb.Options) error {
	syncer := &sourcesSyncer{opts}
	var errs []error
	for _, src := range srcs {
		if err := syncer.syncSource(ctx, src); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func isBare(opts *configpb.Options) bool {
	return opts.GetLayout() == configpb.Options_BARE_LAYOUT
}

func repoRoot(src *Source, opts *configpb.Options) string {
	if src.Path != "" {
		return filepath.Join(opts.GetRoot(), src.Path)
	}
	base := filepath.Join(opts.GetRoot(), src.FullName)
	if isBare(opts) {
		base += ".git"
	}
	return base
}

// SyncStatus captures possible states of the local repository vs its remote.
type SyncStatus int

//go:generate go run github.com/dmarkham/enumer -type=SyncStatus -trimprefix SyncStatus -transform snake-upper
const (
	SyncStatusAbsent SyncStatus = iota
	SyncStatusStale
	SyncStatusFresh
)

type syncable struct {
	source *Source
	folder string
	bare   bool
}

func newSyncable(src *Source, opts *configpb.Options) *syncable {
	folder := repoRoot(src, opts)
	return &syncable{source: src, folder: folder, bare: isBare(opts)}
}

func (t *syncable) trackingRef() string {
	if t.bare {
		return "refs/heads/HEAD"
	}
	return "refs/remotes/origin/HEAD"
}

func (t *syncable) defaultRemoteRef() string {
	return fmt.Sprintf("refs/remotes/origin/%v", t.source.DefaultBranch)
}

func (t *syncable) gitPath(obj string) string {
	if !t.bare {
		obj = filepath.Join(".git", obj)
	}
	return filepath.Join(t.folder, obj)
}

var errSyncFailed = errors.New("sync failed")

func checkSyncStep(err error) {
	if err != nil {
		panic(fmt.Errorf("%w: %v", errSyncFailed, err))
	}
}

type sourcesSyncer struct {
	options *configpb.Options
}

func (f *sourcesSyncer) syncSource(ctx context.Context, src *Source) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if rerr, ok := r.(error); ok && errors.Is(rerr, errSyncFailed) {
				err = rerr
				return
			}
			panic(r)
		}
	}()

	attrs := dataAttrs(slog.String("fullName", src.FullName))
	slog.Debug("Syncing source...", attrs)

	syncable := newSyncable(src, f.options)
	lastSyncedAt := syncableModTime(syncable)
	if lastSyncedAt.IsZero() {
		f.createSyncable(ctx, syncable)
	}
	if src.LastUpdatedAt.IsZero() || lastSyncedAt.Before(src.LastUpdatedAt) {
		f.updateSyncableContents(ctx, syncable)
	}
	f.updateSyncableMetadata(ctx, syncable)
	slog.Info("Synced source.", attrs, errAttr(err))
	return
}

func (f *sourcesSyncer) createSyncable(ctx context.Context, syncable *syncable) {
	checkSyncStep(os.MkdirAll(syncable.folder, 0755))

	// We don't use git clone to avoid having the credentials saved in the repo's config and share
	// more logic with the update function below.
	initArgs := []string{"init", "-b", syncable.source.DefaultBranch}
	if isBare(f.options) {
		initArgs = append(initArgs, "--bare")
	}
	runGitCommand(ctx, syncable.folder, initArgs)
	runGitCommand(ctx, syncable.folder, []string{
		"remote",
		"add",
		"-m",
		syncable.source.DefaultBranch,
		"origin",
		syncable.source.FetchURL,
	})
	slog.Debug("Created syncable repository.", dataAttrs(slog.String("path", syncable.folder)))
}

func (f *sourcesSyncer) updateSyncableContents(ctx context.Context, syncable *syncable) {
	runGitCommand(ctx, syncable.folder, append(syncable.source.fetchFlags, "fetch", "--all"))

	// Update HEAD directly for bare repositories so that gitweb shows the most recent remote commit.
	runGitCommand(ctx, syncable.folder, []string{"update-ref", syncable.trackingRef(), syncable.defaultRemoteRef()})

	if !isBare(f.options) {
		if !fileExists(syncable.gitPath("refs/heads/HEAD")) {
			// No working directory yet.
			runGitCommand(ctx, syncable.folder, []string{"checkout", syncable.source.DefaultBranch})
		} else {
			localRef := runCommand(ctx, syncable.folder, "git", []string{"symbolic-ref", "--short", "HEAD"})
			if localRef == syncable.source.DefaultBranch {
				// TODO: Also check if working directory is clean.
				runGitCommand(ctx, syncable.folder, []string{
					"merge",
					"--ff-only",
					fmt.Sprintf("origin/%v", syncable.source.DefaultBranch),
				})
			}
		}
	}
	slog.Debug("Updated syncable repository contents.", dataAttrs(slog.String("path", syncable.folder)))
}

func (f *sourcesSyncer) updateSyncableMetadata(ctx context.Context, syncable *syncable) {
	runGitCommand(ctx, syncable.folder, []string{"config", "set", "gitweb.url", syncable.source.FetchURL})
	// This allows the remote branches to show up in the summary page's HEADS section.
	runGitCommand(ctx, syncable.folder, []string{"config", "set", "gitweb.extraBranchRefs", "remotes"})
	if desc := syncable.source.Description; desc != "" {
		checkSyncStep(os.WriteFile(syncable.gitPath("description"), []byte(desc), 0644))
	}
	slog.Debug("Updated syncable repository medatada.", dataAttrs(slog.String("path", syncable.folder)))
}

var (
	syncableModTime = func(syncable *syncable) time.Time {
		return fileModTime(syncable.gitPath(syncable.trackingRef()))
	}
	runGitCommand = func(ctx context.Context, cwd string, args []string) {
		runCommand(ctx, cwd, "git", args)
	}
)

func fileModTime(fp string) time.Time {
	info, err := os.Stat(fp)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func fileExists(fp string) bool {
	_, err := os.Stat(fp)
	return !errors.Is(err, fs.ErrNotExist)
}

// runCommand executes a command, panicking if it fails.
func runCommand(ctx context.Context, cwd, name string, args []string) string {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	stdout, err := cmd.StdoutPipe()
	checkSyncStep(err)
	stderr, err := cmd.StderrPipe()
	checkSyncStep(err)
	checkSyncStep(cmd.Start())
	errData, _ := io.ReadAll(stderr)
	if err := cmd.Wait(); err != nil {
		checkSyncStep(fmt.Errorf("%w: %v", err, string(errData)))
	}
	outData, _ := io.ReadAll(stdout)
	return string(outData)
}
