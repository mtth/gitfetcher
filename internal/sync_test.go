package gitfetcher

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncSources(t *testing.T) {
	ctx := context.Background()
	t0 := time.UnixMilli(3600_000)
	t1 := time.UnixMilli(4800_000)

	for key, tc := range map[string]func(*testing.T, map[string]time.Time, fmt.Stringer){
		"no sources": func(t *testing.T, _ map[string]time.Time, _ fmt.Stringer) {
			err := SyncSources(ctx, "/tmp", nil)
			require.NoError(t, err)
		},
		"single missing source": func(t *testing.T, _ map[string]time.Time, out fmt.Stringer) {
			err := SyncSources(ctx, "/tmp", []*Source{{
				Name:          "cool/test",
				FetchURL:      "http://example.com/test",
				defaultBranch: "main",
				LastUpdatedAt: t0,
			}})
			require.NoError(t, err)
			assert.Equal(t, []string{
				"init --bare -b main",
				"remote add -m main origin http://example.com/test",
				"fetch --all",
				"update-ref refs/heads/HEAD refs/remotes/origin/main",
				"config set gitweb.url http://example.com/test",
				"config set gitweb.extraBranchRefs remotes",
			}, strings.Split(strings.TrimSpace(out.String()), "\n"))
		},
		"stale and up-to-date sources": func(t *testing.T, times map[string]time.Time, out fmt.Stringer) {
			times["/tmp/cool/stale.git"] = t0
			times["/tmp/cool/up-to-date.git"] = t0
			err := SyncSources(ctx, "/tmp", []*Source{{
				Name:          "cool/stale",
				FetchURL:      "http://example.com/stale",
				defaultBranch: "main",
				LastUpdatedAt: t1,
			}, {
				Name:          "cool/up-to-date",
				FetchURL:      "http://example.com/up-to-date",
				defaultBranch: "main",
				LastUpdatedAt: t0,
			}})
			require.NoError(t, err)
			assert.Equal(t, []string{
				"fetch --all",
				"update-ref refs/heads/HEAD refs/remotes/origin/main",
				"config set gitweb.url http://example.com/stale",
				"config set gitweb.extraBranchRefs remotes",
				"config set gitweb.url http://example.com/up-to-date",
				"config set gitweb.extraBranchRefs remotes",
			}, strings.Split(strings.TrimSpace(out.String()), "\n"))
		},
	} {
		t.Run(key, func(t *testing.T) {
			var b strings.Builder
			defer swap(&runGitCommand, func(ctx context.Context, cwd string, args []string) error {
				b.WriteString(strings.Join(args, " "))
				b.WriteString("\n")
				return nil
			})()

			ts := make(map[string]time.Time)
			defer swap(&targetModTime, func(tgt *target) time.Time {
				return ts[tgt.folder]
			})()

			tc(t, ts, &b)
		})
	}
}

func TestFileModTime(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		got := fileModTime("./non/existent")
		assert.True(t, got.IsZero())
	})

	t.Run("directory", func(t *testing.T) {
		got := fileModTime(".")
		assert.False(t, got.IsZero())
	})
}

func TestTargetModTime(t *testing.T) {
	got := targetModTime(&target{folder: "./missing"})
	assert.True(t, got.IsZero())
}

func TestRunCommand(t *testing.T) {
	ctx := context.Background()

	t.Run("executable not found", func(t *testing.T) {
		err := runCommand(ctx, ".", "non-existent", nil)
		require.ErrorContains(t, err, "not found")
	})

	t.Run("OK invocation", func(t *testing.T) {
		err := runCommand(ctx, ".", "echo", []string{"bar"})
		require.NoError(t, err)
	})

	t.Run("failed invocation", func(t *testing.T) {
		err := runCommand(ctx, ".", "false", nil)
		require.ErrorContains(t, err, "exit")
	})
}

func TestRunGitCommand(t *testing.T) {
	ctx := context.Background()
	got := runGitCommand(ctx, ".", []string{"status"})
	require.NoError(t, got)
}
