package gitfetcher

import (
	"context"
	"os"
	"testing"

	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindSources_Github(t *testing.T) {
	ctx := context.Background()

	t.Run("single named repo", func(t *testing.T) {
		srcs, err := FindSources(ctx, &configpb.Config{
			Branch: &configpb.Config_Github{
				Github: &configpb.Github{
					Sources: []*configpb.GithubSource{{
						Branch: &configpb.GithubSource_Name{
							Name: "mtth/gitfetcher",
						},
					}},
				},
			},
		})
		require.NoError(t, err)
		assert.Len(t, srcs, 1)
		assert.Equal(t, "mtth/gitfetcher", srcs[0].Name)
	})

	t.Run("invalid token", func(t *testing.T) {
		srcs, err := FindSources(ctx, &configpb.Config{
			Branch: &configpb.Config_Github{
				Github: &configpb.Github{
					Sources: []*configpb.GithubSource{{
						Branch: &configpb.GithubSource_Auth{
							Auth: &configpb.GithubSource_TokenAuth{
								Token: "abc",
							},
						},
					}},
				},
			},
		})
		assert.Nil(t, srcs)
		assert.ErrorIs(t, err, errInvalidGithubToken)
	})

	t.Run("valid token", func(t *testing.T) {
		if os.Getenv("SOURCES_GITHUB_TOKEN") == "" {
			t.SkipNow()
		}

		srcs, err := FindSources(ctx, &configpb.Config{
			Branch: &configpb.Config_Github{
				Github: &configpb.Github{
					Sources: []*configpb.GithubSource{{
						Branch: &configpb.GithubSource_Auth{
							Auth: &configpb.GithubSource_TokenAuth{
								Token:   "$SOURCES_GITHUB_TOKEN",
								Filters: []string{"**"},
							},
						},
					}},
				},
			},
		})
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
