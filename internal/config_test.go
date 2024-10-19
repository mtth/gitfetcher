package gitfetcher

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	for _, tc := range []struct {
		path string
		want Config
	}{
		{
			path: "testdata",
			want: &githubConfig{
				Sources: []githubSourcesConfig{{Match: "foo/*"}, {Match: "bar/here"}},
			},
		},
		{
			path: "testdata/.gitfetcher.great.yaml",
			want: &githubConfig{
				Sources: []githubSourcesConfig{{Match: "great/stuff"}},
			},
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got, err := ParseConfig(tc.path)
			require.NoError(t, err)
			assert.EqualValues(t, tc.want, got)
			assert.Equal(t, "github", got.SourceProvider())
		})
	}

	for _, tc := range []string{
		"testdata/.gitfetcher.empty.yaml",
		"testdata/.gitfetcher.empty-github-sources.yaml",
		"testdata/.gitfetcher.invalid.yaml",
	} {
		t.Run(tc, func(t *testing.T) {
			got, err := ParseConfig(tc)
			assert.Nil(t, got)
			require.ErrorIs(t, err, ErrInvalidConfig)
		})
	}

	for key, tc := range map[string]string{
		"folder": "./non/existent/path",
		"file":   ".",
	} {
		t.Run(fmt.Sprintf("missing %s", key), func(t *testing.T) {
			got, err := ParseConfig(tc)
			assert.Nil(t, got)
			require.ErrorIs(t, err, ErrMissingConfig)
		})
	}
}
