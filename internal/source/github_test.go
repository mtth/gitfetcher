package source

import (
	"context"
	"testing"

	"github.com/google/go-github/v66/github"
	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"github.com/mtth/gitfetcher/internal/effect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addr[T any](t T) *T { return &t }

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

func TestRealGithubClient_urlSource(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	token := effect.RequireEvar(t, "SOURCE_GITHUB_TOKEN")

	t.Run("public URL origin", func(t *testing.T) {
		client := newRealGithubClient()
		src, err := client.urlSource(ctx, &configpb.UrlOrigin{
			Url: "https://github.com/mtth/gitfetcher",
		})
		require.NoError(t, err)
		assert.Equal(t, "mtth/gitfetcher", src.FullName)
		assert.Equal(t, "main", src.DefaultBranch)
	})

	t.Run("invalid token", func(t *testing.T) {
		client := newRealGithubClient()
		_, err := client.tokenSources(ctx, &configpb.GithubTokenOrigin{Token: "abc"})
		require.ErrorIs(t, err, errInvalidGithubToken)
	})

	t.Run("valid token", func(t *testing.T) {
		client := newRealGithubClient()
		got, err := client.tokenSources(ctx, &configpb.GithubTokenOrigin{Token: token})
		require.NoError(t, err)
		assert.NotEmpty(t, got)
	})
}
