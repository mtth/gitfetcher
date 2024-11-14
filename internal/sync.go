package gitfetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
)

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

// GetSyncStatus returns the current SyncStatus of a source.
func GetSyncStatus(src *Source, opts *configpb.Options) SyncStatus {
	target := newTarget(src, opts)
	lastSyncedAt := targetModTime(target)
	if lastSyncedAt.IsZero() {
		return SyncStatusAbsent
	}
	if src.LastUpdatedAt.IsZero() || lastSyncedAt.Before(src.LastUpdatedAt) {
		return SyncStatusStale
	}
	return SyncStatusFresh
}

func isBare(opts *configpb.Options) bool {
	return opts.GetLayout() == configpb.Options_BARE_LAYOUT
}

func repoRoot(src *Source, opts *configpb.Options) string {
	base := filepath.Join(opts.GetRoot(), src.Name)
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

type sourcesSyncer struct {
	options *configpb.Options
}

type target struct {
	source *Source
	folder string
	bare   bool
}

func newTarget(src *Source, opts *configpb.Options) *target {
	folder := repoRoot(src, opts)
	return &target{source: src, folder: folder, bare: isBare(opts)}
}

func (t *target) trackingRef() string {
	if t.bare {
		return "refs/heads/HEAD"
	}
	return "refs/remotes/origin/HEAD"
}

func (t *target) defaultRemoteRef() string {
	return fmt.Sprintf("refs/remotes/origin/%v", t.source.DefaultBranch)
}

func (t *target) gitPath(obj string) string {
	if !t.bare {
		obj = filepath.Join(".git", obj)
	}
	return filepath.Join(t.folder, obj)
}

func (f *sourcesSyncer) syncSource(ctx context.Context, src *Source) error {
	attrs := dataAttrs(slog.String("name", src.Name))
	slog.Debug("Syncing source...", attrs)

	target := newTarget(src, f.options)
	lastSyncedAt := targetModTime(target)
	if lastSyncedAt.IsZero() {
		if err := f.createTarget(ctx, target); err != nil {
			return err
		}
	}
	if src.LastUpdatedAt.IsZero() || lastSyncedAt.Before(src.LastUpdatedAt) {
		if err := f.updateTargetContents(ctx, target); err != nil {
			return err
		}
	}
	err := f.updateTargetMetadata(ctx, target)
	slog.Info("Synced source.", attrs, errAttr(err))
	return err
}

func (f *sourcesSyncer) createTarget(ctx context.Context, target *target) error {
	if err := os.MkdirAll(target.folder, 0755); err != nil {
		return err
	}
	// We don't use git clone to avoid having the credentials saved in the repo's config and share
	// more logic with the update function below.
	initArgs := []string{"init", "-b", target.source.DefaultBranch}
	if isBare(f.options) {
		initArgs = append(initArgs, "--bare")
	}
	if err := runGitCommand(ctx, target.folder, initArgs); err != nil {
		return err
	}
	if err := runGitCommand(ctx, target.folder, []string{
		"remote",
		"add",
		"-m",
		target.source.DefaultBranch,
		"origin",
		target.source.FetchURL,
	}); err != nil {
		return err
	}
	slog.Debug("Created target repository.", dataAttrs(slog.String("path", target.folder)))
	return nil
}

func (f *sourcesSyncer) updateTargetContents(ctx context.Context, target *target) error {
	if err := runGitCommand(ctx, target.folder, append(
		target.source.fetchFlags,
		"fetch",
		"--all",
	)); err != nil {
		return err
	}

	// Update HEAD directly for bare repositories so that gitweb shows the most recent remote commit.
	if err := runGitCommand(ctx, target.folder, []string{
		"update-ref",
		target.trackingRef(),
		target.defaultRemoteRef(),
	}); err != nil {
		return err
	}

	if !isBare(f.options) {
		if !fileExists(target.gitPath("refs/heads/HEAD")) {
			// No working directory yet.
			if err := runGitCommand(ctx, target.folder, []string{"checkout", target.source.DefaultBranch}); err != nil {
				return err
			}
		} else {
			localRef, err := runCommand(ctx, target.folder, "git", []string{"symbolic-ref", "--short", "HEAD"})
			if err != nil {
				return err
			}
			if localRef == target.source.DefaultBranch {
				// TODO: Also check if working directory is clean.
				if err := runGitCommand(ctx, target.folder, []string{
					"merge",
					"--ff-only",
					fmt.Sprintf("origin/%v", target.source.DefaultBranch),
				}); err != nil {
					return err
				}
			}
		}
	}
	slog.Debug("Updated target repository contents.", dataAttrs(slog.String("path", target.folder)))
	return nil
}

func (f *sourcesSyncer) updateTargetMetadata(ctx context.Context, target *target) error {
	errs := []error{
		runGitCommand(ctx, target.folder, []string{"config", "set", "gitweb.url", target.source.FetchURL}),
		// This allows the remote branches to show up in the summary page's HEADS section.
		runGitCommand(ctx, target.folder, []string{"config", "set", "gitweb.extraBranchRefs", "remotes"}),
	}
	if desc := target.source.Description; desc != "" {
		errs = append(errs, os.WriteFile(target.gitPath("description"), []byte(desc), 0644))
	}
	err := errors.Join(errs...)
	slog.Debug("Updated target repository medatada.", dataAttrs(slog.String("path", target.folder)))
	return err
}

var (
	targetModTime = func(target *target) time.Time {
		return fileModTime(target.gitPath(target.trackingRef()))
	}
	runGitCommand = func(ctx context.Context, cwd string, args []string) error {
		_, err := runCommand(ctx, cwd, "git", args)
		return err
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
	return !os.IsNotExist(err)
}

func runCommand(ctx context.Context, cwd, name string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	errData, _ := io.ReadAll(stderr)
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("%w: %v", err, string(errData))
	}
	outData, _ := io.ReadAll(stdout)
	return string(outData), nil
}
