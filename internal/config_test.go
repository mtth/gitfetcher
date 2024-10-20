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
				Branch: &configpb.Config_Github{
					Github: &configpb.Github{
						Sources: []*configpb.GithubSource{{
							Branch: &configpb.GithubSource_Name{
								Name: "mtth/gitfetcher",
							},
						}, {
							Branch: &configpb.GithubSource_Auth{
								Auth: &configpb.GithubSource_TokenAuth{
									Token: "$GITHUB_TOKEN",
								},
							},
						}},
					},
				},
			},
		},
		{
			path: "testdata/.gitfetcher.great",
			want: &configpb.Config{
				Branch: &configpb.Config_Github{
					Github: &configpb.Github{
						Sources: []*configpb.GithubSource{{
							Branch: &configpb.GithubSource_Name{
								Name: "great/stuff",
							},
						}},
					},
				},
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
		"testdata/.gitfetcher.empty",
		"testdata/.gitfetcher.invalid",
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
