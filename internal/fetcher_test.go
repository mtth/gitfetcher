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

func TestSourceFetcher(t *testing.T) {
	ctx := context.Background()
	t0 := time.UnixMilli(3600_000)
	t1 := time.UnixMilli(4800_000)

	for key, tc := range map[string]func(*testing.T, map[string]time.Time, fmt.Stringer){
		"no sources": func(t *testing.T, _ map[string]time.Time, _ fmt.Stringer) {
			fetcher := &realSourceFetcher{"/tmp", inlineSources(nil)}
			err := fetcher.FetchSources(ctx)
			require.NoError(t, err)
		},
		"single missing source": func(t *testing.T, _ map[string]time.Time, out fmt.Stringer) {
			fetcher := &realSourceFetcher{"/tmp", inlineSources([]*source{{
				name:          "cool/test",
				fetchURL:      "http://example.com/test",
				defaultBranch: "main",
				lastUpdatedAt: t0,
			}})}
			err := fetcher.FetchSources(ctx)
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
			fetcher := &realSourceFetcher{"/tmp", inlineSources([]*source{{
				name:          "cool/stale",
				fetchURL:      "http://example.com/stale",
				defaultBranch: "main",
				lastUpdatedAt: t1,
			}, {
				name:          "cool/up-to-date",
				fetchURL:      "http://example.com/up-to-date",
				defaultBranch: "main",
				lastUpdatedAt: t0,
			}})}
			err := fetcher.FetchSources(ctx)
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

type inlineSources []*source

func (f inlineSources) findSources(context.Context) ([]*source, error) {
	return ([]*source)(f), nil
}
