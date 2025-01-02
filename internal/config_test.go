package gitfetcher

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"github.com/mtth/gitfetcher/internal/effect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestReadConfig(t *testing.T) {
	defer effect.Swap(&filepathAbs, func(p string) (string, error) { return "/root/" + p, nil })()

	for _, tc := range []struct {
		path string
		want *configpb.Config
	}{
		{
			path: "testdata/.gitfetcher.conf",
			want: &configpb.Config{
				Options: &configpb.Options{Root: "/root/testdata/projects"},
				Sources: []*configpb.Source{{
					Branch: &configpb.Source_FromUrl{
						FromUrl: &configpb.UrlSource{
							Url: "https://github.com/mtth/gitfetcher",
						},
					},
				}, {
					Branch: &configpb.Source_FromGithubToken{
						FromGithubToken: &configpb.GithubTokenSource{
							Token: "$GITHUB_TOKEN",
						},
					},
				}},
			},
		},
		{
			path: "testdata/.gitfetcher.great.conf",
			want: &configpb.Config{
				Sources: []*configpb.Source{{
					Branch: &configpb.Source_FromGithubToken{
						FromGithubToken: &configpb.GithubTokenSource{
							Token:          "secret-token",
							Filters:        []string{"foo/*"},
							RemoteProtocol: configpb.RemoteProtocol_SSH_REMOTE_PROTOCOL,
						},
					},
				}},
				Options: &configpb.Options{Root: "/root/testdata"},
			},
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got, err := ReadConfig(tc.path)
			require.NoError(t, err)
			assert.Empty(t, cmp.Diff(tc.want, got, protocmp.Transform()))
		})
	}

	for _, tc := range []string{
		"testdata/.gitfetcher.invalid.conf",
	} {
		t.Run(tc, func(t *testing.T) {
			got, err := ReadConfig(tc)
			assert.Nil(t, got)
			require.ErrorIs(t, err, errInvalidConfig)
		})
	}

	for key, tc := range map[string]string{
		"folder": "./non/existent/path",
		"file":   ".",
	} {
		t.Run(fmt.Sprintf("missing %s", key), func(t *testing.T) {
			got, err := ReadConfig(tc)
			assert.Nil(t, got)
			require.ErrorIs(t, err, errMissingConfig)
		})
	}
}

func TestFindConfig(t *testing.T) {
	t.Run("implicit", func(t *testing.T) {
		got, err := FindConfig(".")
		require.NoError(t, err)
		assert.NotEmpty(t, got.GetOptions().GetRoot())
	})

	t.Run("explicit", func(t *testing.T) {
		got, err := FindConfig("testdata")
		require.NoError(t, err)
		assert.Len(t, got.GetSources(), 2)
	})
}
