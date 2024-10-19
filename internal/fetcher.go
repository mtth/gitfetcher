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

type SourceFetcher interface {
	FetchSources(ctx context.Context) error
}

func NewSourceFetcher(root string, cfg Config) (SourceFetcher, error) {
	finder, err := newSourceFinder(cfg)
	if err != nil {
		return nil, err
	}
	return &realSourceFetcher{root, finder}, nil
}

type realSourceFetcher struct {
	root   string
	finder sourceFinder
}

func (f *realSourceFetcher) FetchSources(ctx context.Context) error {
	srcs, err := f.finder.findSources(ctx)
	if err != nil {
		return err
	}

	for _, src := range srcs {
		attrs := slog.Group("data", slog.String("name", src.name), slog.String("url", src.fetchURL))
		if err := f.syncSource(ctx, src); err != nil {
			slog.Error("Unable to sync source.", attrs, slog.Any("err", err))
		} else {
			slog.Info("Synced source.", attrs)
		}
	}
	return nil
}

type target struct {
	folder string
	source *source
}

var errUnexpectedSourceStatus = errors.New("unexpected source status")

func (f *realSourceFetcher) syncSource(ctx context.Context, src *source) error {
	target := &target{
		source: src,
		folder: filepath.Join(f.root, src.name) + ".git",
	}

	lastSyncedAt := targetModTime(target)
	if lastSyncedAt.IsZero() {
		if err := f.createTarget(ctx, target); err != nil {
			return err
		}
	}
	if lastSyncedAt.Before(src.lastUpdatedAt) {
		if err := f.updateTargetContents(ctx, target); err != nil {
			return err
		}
	}
	return f.updateTargetMetadata(ctx, target)
}

func (f *realSourceFetcher) createTarget(ctx context.Context, target *target) error {
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
		target.source.fetchURL,
	})
}

func (f *realSourceFetcher) updateTargetContents(ctx context.Context, target *target) error {
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

func (f *realSourceFetcher) updateTargetMetadata(ctx context.Context, target *target) error {
	errs := []error{
		runGitCommand(ctx, target.folder, []string{"config", "set", "gitweb.url", target.source.fetchURL}),
		// This allows the remote branches to show up in the summary page's HEADS section.
		runGitCommand(ctx, target.folder, []string{"config", "set", "gitweb.extraBranchRefs", "remotes"}),
	}
	if desc := target.source.description; desc != "" {
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
