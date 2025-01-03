package source

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-github/v66/github"
	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSources_StandardURL(t *testing.T) {
	ctx := context.Background()

	t.Run("single named repo", func(t *testing.T) {
		srcs, err := Load(ctx, []*configpb.Source{{
			Branch: &configpb.Source_FromUrl{
				FromUrl: &configpb.UrlSource{
					Url:           "https://gitlab.archlinux.org/archlinux/devtools.git",
					DefaultBranch: "master",
				},
			},
		}})
		require.NoError(t, err)
		assert.Len(t, srcs, 1)
		assert.Equal(t, "archlinux/devtools", srcs[0].FullName)
		assert.Equal(t, "master", srcs[0].DefaultBranch)
	})

	t.Run("invalid URL", func(t *testing.T) {
		srcs, err := Load(ctx, []*configpb.Source{{
			Branch: &configpb.Source_FromUrl{
				FromUrl: &configpb.UrlSource{
					Url: "::/invalid.git",
				},
			},
		}})
		assert.Nil(t, srcs)
		assert.ErrorIs(t, err, errInvalidURL)
	})
}

func TestLoadSources_Github(t *testing.T) {
	ctx := context.Background()

	t.Run("single named repo", func(t *testing.T) {
		srcs, err := Load(ctx, []*configpb.Source{{
			Branch: &configpb.Source_FromUrl{
				FromUrl: &configpb.UrlSource{
					Url: "https://github.com/mtth/gitfetcher",
				},
			},
		}})
		require.NoError(t, err)
		assert.Len(t, srcs, 1)
		assert.Equal(t, "mtth/gitfetcher", srcs[0].FullName)
		assert.Equal(t, "main", srcs[0].DefaultBranch)
	})

	t.Run("invalid token", func(t *testing.T) {
		srcs, err := Load(ctx, []*configpb.Source{{
			Branch: &configpb.Source_FromGithubToken{
				FromGithubToken: &configpb.GithubTokenSource{
					Token: "abc",
				},
			},
		}})
		assert.Nil(t, srcs)
		assert.ErrorIs(t, err, errInvalidGithubToken)
	})

	t.Run("valid token", func(t *testing.T) {
		if os.Getenv("SOURCES_GITHUB_TOKEN") == "" {
			t.SkipNow()
		}

		srcs, err := Load(ctx, []*configpb.Source{{
			Branch: &configpb.Source_FromGithubToken{
				FromGithubToken: &configpb.GithubTokenSource{
					Token:          "$SOURCES_GITHUB_TOKEN",
					RemoteProtocol: configpb.RemoteProtocol_SSH_REMOTE_PROTOCOL,
				},
			},
		}})
		require.NoError(t, err)
		assert.NotEmpty(t, srcs)
	})
}

func TestNamePredicate(t *testing.T) {
	t.Run("invalid", func(t *testing.T) {
		got, err := newNamePredicate([]string{"["})
		assert.Nil(t, got)
		assert.ErrorContains(t, err, "end of input")
	})

	t.Run("empty", func(t *testing.T) {
		got, err := newNamePredicate(nil)
		require.NoError(t, err)
		assert.True(t, got.accept(""))
	})

	t.Run("glob", func(t *testing.T) {
		got, err := newNamePredicate([]string{"foo/*"})
		require.NoError(t, err)
		assert.True(t, got.accept("foo/bar"))
		assert.True(t, got.accept("foo/.baz"))
		assert.False(t, got.accept("bar/baz"))
	})
}

func TestGithubSourcePath(t *testing.T) {
	for key, tc := range map[string]struct {
		tpl   string
		repo  *github.Repository
		path  string
		isErr bool
	}{
		"empty template":  {},
		"static template": {tpl: "foo", path: "foo"},
		"dynamic template": {
			tpl: "{{ .Owner }}/{{ .Name }}",
			repo: &github.Repository{
				Owner: &github.User{Name: addr("ann")},
				Name:  addr("bar"),
			},
			path: "ann/bar",
		},
		"invalid template": {tpl: "{{ .Unterminated", path: "foo", isErr: true},
	} {
		t.Run(key, func(t *testing.T) {
			path, err := githubSourcePath(tc.tpl, tc.repo)
			if tc.isErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, path, tc.path)
			}
		})
	}
}

func addr[T any](t T) *T { return &t }
