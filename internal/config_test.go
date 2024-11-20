package gitfetcher

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestParseConfig(t *testing.T) {
	for _, tc := range []struct {
		path string
		want *configpb.Config
	}{
		{
			path: "testdata",
			want: &configpb.Config{
				Options: &configpb.Options{Root: "testdata/projects"},
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
							RemoteProtocol: configpb.GithubTokenSource_SSH_REMOTE_PROTOCOL,
						},
					},
				}},
				Options: &configpb.Options{Root: "testdata"},
			},
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got, err := ParseConfig(tc.path)
			require.NoError(t, err)
			assert.Empty(t, cmp.Diff(tc.want, got, protocmp.Transform()))
		})
	}

	for _, tc := range []string{
		"testdata/.gitfetcher.empty.conf",
		"testdata/.gitfetcher.invalid.conf",
	} {
		t.Run(tc, func(t *testing.T) {
			got, err := ParseConfig(tc)
			assert.Nil(t, got)
			require.ErrorIs(t, err, errInvalidConfig)
		})
	}

	for key, tc := range map[string]string{
		"folder": "./non/existent/path",
		"file":   ".",
	} {
		t.Run(fmt.Sprintf("missing %s", key), func(t *testing.T) {
			got, err := ParseConfig(tc)
			assert.Nil(t, got)
			require.ErrorIs(t, err, errMissingConfig)
		})
	}
}
