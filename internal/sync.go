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

func SyncSources(ctx context.Context, root string, srcs []*Source) error {
	syncer := &realSourcesSyncer{root}
	var errs []error
	for _, src := range srcs {
		attrs := slog.Group("data", slog.String("name", src.Name), slog.String("url", src.FetchURL))
		if err := syncer.syncSource(ctx, src); err != nil {
			slog.Error("Unable to sync source.", attrs, slog.Any("err", err))
		} else {
			slog.Info("Synced source.", attrs)
		}
	}
	return errors.Join(errs...)
}

type realSourcesSyncer struct {
	root string
}

type target struct {
	folder string
	source *Source
}

func (f *realSourcesSyncer) syncSource(ctx context.Context, src *Source) error {
	target := &target{
		source: src,
		folder: filepath.Join(f.root, src.Name) + ".git",
	}

	lastSyncedAt := targetModTime(target)
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
	return f.updateTargetMetadata(ctx, target)
}

func (f *realSourcesSyncer) createTarget(ctx context.Context, target *target) error {
	if err := os.MkdirAll(target.folder, 0755); err != nil {
		return err
	}
	// We don't use git clone to avoid having the credentials saved in the repo's config and share
	// more logic with the update function below.
	if err := runGitCommand(ctx, target.folder, []string{
		"init",
		"--bare",
		"-b",
		target.source.defaultBranch,
	}); err != nil {
		return err
	}
	return runGitCommand(ctx, target.folder, []string{
		"remote",
		"add",
		"-m",
		target.source.defaultBranch,
		"origin",
		target.source.FetchURL,
	})
}

func (f *realSourcesSyncer) updateTargetContents(ctx context.Context, target *target) error {
	if err := runGitCommand(ctx, target.folder, append(
		target.source.fetchFlags,
		"fetch",
		"--all",
	)); err != nil {
		return err
	}
	// Update HEAD so that gitweb shows the most recent commit.
	return runGitCommand(ctx, target.folder, []string{
		"update-ref",
		"refs/heads/HEAD",
		fmt.Sprintf("refs/remotes/origin/%v", target.source.defaultBranch),
	})
}

func (f *realSourcesSyncer) updateTargetMetadata(ctx context.Context, target *target) error {
	errs := []error{
		runGitCommand(ctx, target.folder, []string{"config", "set", "gitweb.url", target.source.FetchURL}),
		// This allows the remote branches to show up in the summary page's HEADS section.
		runGitCommand(ctx, target.folder, []string{"config", "set", "gitweb.extraBranchRefs", "remotes"}),
	}
	if desc := target.source.Description; desc != "" {
		errs = append(errs, os.WriteFile(filepath.Join(target.folder, "description"), []byte(desc), 0644))
	}
	return errors.Join(errs...)
}

var (
	targetModTime = func(tgt *target) time.Time {
		return fileModTime(filepath.Join(tgt.folder, "refs/heads"))
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
