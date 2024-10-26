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
)

// Sync syncs local copies in the root folder of each source. Missing local repositories will be
// created, others will be updated as needed.
func Sync(ctx context.Context, root string, srcs []*Source) error {
	syncer := &sourcesSyncer{root}
	var errs []error
	for _, src := range srcs {
		if err := syncer.syncSource(ctx, src); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// GetSyncStatus returns the current SyncStatus of a source.
func GetSyncStatus(root string, src *Source) SyncStatus {
	lastSyncedAt := repoModTime(repoRoot(root, src))
	if lastSyncedAt.IsZero() {
		return SyncStatusAbsent
	}
	if lastSyncedAt.Before(src.LastUpdatedAt) {
		return SyncStatusStale
	}
	return SyncStatusFresh
}

func repoRoot(root string, src *Source) string {
	return filepath.Join(root, src.Name) + ".git"
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
	root string
}

type target struct {
	folder string
	source *Source
}

func (f *sourcesSyncer) syncSource(ctx context.Context, src *Source) error {
	attrs := dataAttrs(slog.String("name", src.Name))
	slog.Debug("Syncing source...", attrs)

	target := &target{
		source: src,
		folder: repoRoot(f.root, src),
	}

	lastSyncedAt := repoModTime(target.folder)
	if lastSyncedAt.IsZero() {
		if err := f.createTarget(ctx, target); err != nil {
			return err
		}
	}
	if lastSyncedAt.Before(src.LastUpdatedAt) {
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
	if err := runGitCommand(ctx, target.folder, []string{
		"init",
		"--bare",
		"-b",
		target.source.DefaultBranch,
	}); err != nil {
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
	// Update HEAD so that gitweb shows the most recent commit.
	if err := runGitCommand(ctx, target.folder, []string{
		"update-ref",
		"refs/heads/HEAD",
		fmt.Sprintf("refs/remotes/origin/%v", target.source.DefaultBranch),
	}); err != nil {
		return err
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
		errs = append(errs, os.WriteFile(filepath.Join(target.folder, "description"), []byte(desc), 0644))
	}
	err := errors.Join(errs...)
	slog.Debug("Updated target repository medatada.", dataAttrs(slog.String("path", target.folder)))
	return err
}

var (
	repoModTime = func(fp string) time.Time {
		return fileModTime(filepath.Join(fp, "refs/heads"))
	}
	runGitCommand = func(ctx context.Context, cwd string, args []string) error {
		return runCommand(ctx, cwd, "git", args)
	}
)

func fileModTime(fp string) time.Time {
	info, err := os.Stat(fp)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func runCommand(ctx context.Context, cwd, name string, args []string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	txt, _ := io.ReadAll(stderr)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%w: %v", err, string(txt))
	}
	return nil
}
