package source

import (
	"context"
	"errors"
	"testing"

	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"github.com/mtth/gitfetcher/internal/effect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeGithubClient struct {
	mock.Mock
}

func (c *fakeGithubClient) urlSource(ctx context.Context, origin *configpb.UrlOrigin) (*Source, error) {
	args := c.Called(ctx, origin)
	return args.Get(0).(*Source), args.Error(1)
}

func (c *fakeGithubClient) tokenSources(ctx context.Context, origin *configpb.GithubTokenOrigin) ([]*Source, error) {
	args := c.Called(ctx, origin)
	return args.Get(0).([]*Source), args.Error(1)
}

func TestLoad_UrlOrigin(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid URL", func(t *testing.T) {
		srcs, err := Load(ctx, []*configpb.Origin{{
			Branch: &configpb.Origin_FromUrl{
				FromUrl: &configpb.UrlOrigin{
					Url: "::/invalid.git",
				},
			},
		}})
		assert.Nil(t, srcs)
		assert.ErrorIs(t, err, errInvalidURL)
	})

	t.Run("GitLab URL", func(t *testing.T) {
		srcs, err := Load(ctx, []*configpb.Origin{{
			Branch: &configpb.Origin_FromUrl{
				FromUrl: &configpb.UrlOrigin{
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

	t.Run("GitHub URL OK", func(t *testing.T) {
		origin := &configpb.UrlOrigin{Url: "https://github.com/mtth/gitfetcher"}
		src := &Source{FullName: "abc"}

		var client fakeGithubClient
		client.On("urlSource", ctx, origin).Return(src, nil)
		defer effect.Swap(&newGithubClient, func() githubClient { return &client })()

		got, err := Load(ctx, []*configpb.Origin{{
			Branch: &configpb.Origin_FromUrl{FromUrl: origin},
		}})
		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, src, got[0])

		client.AssertExpectations(t)
	})

	t.Run("GitHub URL error", func(t *testing.T) {
		origin := &configpb.UrlOrigin{Url: "https://github.com/mtth/gitfetcher"}
		boom := errors.New("boom")

		var client fakeGithubClient
		client.On("urlSource", ctx, origin).Return((*Source)(nil), boom)
		defer effect.Swap(&newGithubClient, func() githubClient { return &client })()

		_, err := Load(ctx, []*configpb.Origin{{
			Branch: &configpb.Origin_FromUrl{FromUrl: origin},
		}})
		require.ErrorIs(t, err, boom)

		client.AssertExpectations(t)
	})

	t.Run("GitHub token OK", func(t *testing.T) {
		origin := &configpb.GithubTokenOrigin{Token: "token"}
		srcs := []*Source{{FullName: "abc"}, {FullName: "def"}}

		var client fakeGithubClient
		client.On("tokenSources", ctx, origin).Return(srcs, nil)
		defer effect.Swap(&newGithubClient, func() githubClient { return &client })()

		got, err := Load(ctx, []*configpb.Origin{{
			Branch: &configpb.Origin_FromGithubToken{FromGithubToken: origin},
		}})
		require.NoError(t, err)
		assert.Equal(t, got, srcs)

		client.AssertExpectations(t)
	})

	t.Run("GitHub token error", func(t *testing.T) {
		origin := &configpb.GithubTokenOrigin{Token: "token"}
		boom := errors.New("boom")

		var client fakeGithubClient
		client.On("tokenSources", ctx, origin).Return(([]*Source)(nil), boom)
		defer effect.Swap(&newGithubClient, func() githubClient { return &client })()

		_, err := Load(ctx, []*configpb.Origin{{
			Branch: &configpb.Origin_FromGithubToken{FromGithubToken: origin},
		}})
		require.ErrorIs(t, err, boom)

		client.AssertExpectations(t)
	})
}
