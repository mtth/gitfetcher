package gitfetcher

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"github.com/mtth/gitfetcher/internal/effect"
	"github.com/mtth/gitfetcher/internal/fspath"
	"github.com/mtth/gitfetcher/internal/source"
	"github.com/mtth/gitfetcher/internal/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncable_Sync(t *testing.T) {
	ctx := context.Background()
	t0 := time.UnixMilli(3600_000)
	t1 := time.UnixMilli(4800_000)

	for key, tc := range map[string]func(*testing.T, fmt.Stringer){
		"single bare missing source": func(t *testing.T, out fmt.Stringer) {
			syncables, err := GatherSyncables(
				nil,
				[]source.Source{{
					FullName:      "cool/test",
					FetchURL:      "http://example.com/test",
					DefaultBranch: "main",
					LastUpdatedAt: t0,
					RelPath:       "foo",
				}},
				"/tmp",
				configpb.Options_BARE_LAYOUT,
			)
			require.NoError(t, err)
			require.Len(t, syncables, 1)

			err = syncables[0].Sync(ctx)
			require.NoError(t, err)
			assert.Equal(t, []string{
				"init -b main --bare",
				"remote add origin http://example.com/test",
				"config set gitweb.url http://example.com/test",
				"config set gitweb.extraBranchRefs remotes",
				"fetch --all",
				"update-ref refs/heads/HEAD refs/remotes/origin/main",
			}, strings.Split(strings.TrimSpace(out.String()), "\n"))
		},
		"single missing source": func(t *testing.T, out fmt.Stringer) {
			syncables, err := GatherSyncables(
				nil,
				[]source.Source{{
					FullName:      "cool/test",
					FetchURL:      "http://example.com/test",
					DefaultBranch: "main",
					LastUpdatedAt: t0,
				}},
				"/tmp",
				configpb.Options_DEFAULT_LAYOUT,
			)
			require.NoError(t, err)
			require.Len(t, syncables, 1)

			err = syncables[0].Sync(ctx)
			require.NoError(t, err)
			assert.Equal(t, []string{
				"init -b main",
				"remote add origin http://example.com/test",
				"config set gitweb.url http://example.com/test",
				"config set gitweb.extraBranchRefs remotes",
				"fetch --all",
				"checkout main",
			}, strings.Split(strings.TrimSpace(out.String()), "\n"))
		},
		"stale and up-to-date sources": func(t *testing.T, out fmt.Stringer) {
			syncables, err := GatherSyncables(
				[]target.Target{fakeTarget{
					path:                "/tmp/cool/stale",
					remoteLastUpdatedAt: t0,
				}, fakeTarget{
					path:                "/tmp/cool/up-to-date",
					remoteLastUpdatedAt: t0,
				}},
				[]source.Source{{
					FullName:      "cool/stale",
					FetchURL:      "http://example.com/stale",
					DefaultBranch: "main",
					LastUpdatedAt: t1,
				}, {
					FullName:      "cool/up-to-date",
					FetchURL:      "http://example.com/up-to-date",
					DefaultBranch: "main",
					LastUpdatedAt: t0,
				}},
				"/tmp",
				configpb.Options_DEFAULT_LAYOUT,
			)
			require.NoError(t, err)
			require.Len(t, syncables, 2)

			err = syncables[0].Sync(ctx)
			require.NoError(t, err)
			err = syncables[1].Sync(ctx)
			require.NoError(t, err)

			assert.Equal(t, []string{
				"config set gitweb.url http://example.com/stale",
				"config set gitweb.extraBranchRefs remotes",
				"fetch --all",
				"checkout main",
				"config set gitweb.url http://example.com/up-to-date",
				"config set gitweb.extraBranchRefs remotes",
			}, strings.Split(strings.TrimSpace(out.String()), "\n"))
		},
	} {
		t.Run(key, func(t *testing.T) {
			var b strings.Builder
			defer effect.Swap(&runGitCommand, func(ctx context.Context, cwd string, args []string) {
				b.WriteString(strings.Join(args, " "))
				b.WriteString("\n")
			})()
			tc(t, &b)
		})
	}
}

func TestGetSyncStatus(t *testing.T) {
	t.Run("missing source", func(t *testing.T) {
		syncable := Syncable{
			source: &source.Source{
				FullName:      "cool/test",
				FetchURL:      "http://example.com/up-to-date",
				DefaultBranch: "main",
			},
		}
		got := syncable.SyncStatus()
		assert.Equal(t, SyncStatusMissing, got)
	})
}

func TestRunCommand(t *testing.T) {
	ctx := context.Background()

	t.Run("executable not found", func(t *testing.T) {
		require.Panics(t, func() { runCommand(ctx, ".", "non-existent", nil) })
	})

	t.Run("OK invocation", func(t *testing.T) {
		require.NotPanics(t, func() { runCommand(ctx, ".", "echo", []string{"bar"}) })
	})

	t.Run("failed invocation", func(t *testing.T) {
		require.Panics(t, func() { runCommand(ctx, ".", "false", nil) })
	})
}

func TestRunGitCommand(t *testing.T) {
	ctx := context.Background()
	require.NotPanics(t, func() { runGitCommand(ctx, ".", []string{"status"}) })
}

/*
func TestSyncable_RootDir(t *testing.T) {
	for key, tc := range map[string]struct {
		syncable Syncable
		want     string
	}{
		"bare target": {
			syncable: Syncable{
				target: &Target{Path: "/foo/bar/.git", IsBare: false},
			},
		},
	} {
		t.Run(key, func(t *testing.T) {

		})
	}
}
*/

type fakeTarget struct {
	path                fspath.Local
	remoteLastUpdatedAt time.Time
}

func (t fakeTarget) GitDir() fspath.Local           { return filepath.Join(t.WorkDir(), ".git") }
func (t fakeTarget) WorkDir() fspath.Local          { return filepath.FromSlash(t.path) }
func (t fakeTarget) RemoteLastUpdatedAt() time.Time { return t.remoteLastUpdatedAt }
