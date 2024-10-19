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
								Token: "$SOURCES_GITHUB_TOKEN",
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
