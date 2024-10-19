package gitfetcher

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmptyConfig(t *testing.T) {
	finder, err := newSourceFinder(nil)
	assert.Nil(t, finder)
	assert.ErrorIs(t, err, errUnexpectedConfig)
}

func TestGithubSourcesFinder(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		defer swap(&getenv, func(string) string { return "" })()

		finder, err := newSourceFinder(&githubConfig{})
		assert.Nil(t, finder)
		assert.ErrorIs(t, err, errMissingGithubToken)
	})

	t.Run("invalid token", func(t *testing.T) {
		defer swap(&getenv, func(string) string { return "invalid" })()

		finder, err := newSourceFinder(&githubConfig{})
		require.NoError(t, err)

		srcs, err := finder.findSources(context.Background())
		assert.Nil(t, srcs)
		assert.ErrorIs(t, err, errInvalidGithubToken)
	})

	t.Run("valid token", func(t *testing.T) {
		token := os.Getenv("SOURCES_GITHUB_TOKEN")
		if token == "" {
			t.SkipNow()
		}

		defer swap(&getenv, func(string) string { return token })()

		ctx := context.Background()
		finder, err := newSourceFinder(&githubConfig{
			Sources: []githubSourcesConfig{{Match: "**"}},
		})
		require.NoError(t, err)

		srcs, err := finder.findSources(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, srcs)
	})
}
