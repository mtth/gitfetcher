// Package source loads remote information about repositories.
package source

import (
	"maps"
	urlpkg "net/url"
	"slices"
	"strings"
	"time"

	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"github.com/mtth/gitfetcher/internal/fspath"
)

// Source captures information about a repository to be mirrored.
type Source struct {
	// Qualified repository name, typically $owner/$name. Non-empty.
	FullName string
	// Optional human-readable description. May be empty.
	Description string
	// Default branch. May be empty.
	DefaultBranch string
	// Local relative path override. May be empty.
	RelPath fspath.POSIX
	// Last time the remote repository was updated. Zero if unknown.
	LastUpdatedAt time.Time
	// URL used to fetch updates. Non-empty.
	FetchURL string
	// Git flags used to fetch updates. This is particularly useful for authentication.
	FetchFlags []string
	// All URLs associated with this repository. This is used to map existing local repositories to
	// sources.
	URLs []string
}

type options struct {
	defaultBranch  string
	path           fspath.POSIX
	fetchFlags     []string
	remoteProtocol configpb.RemoteProtocol
}

func fromURL(u *urlpkg.URL, opts options) *Source {
	return &Source{
		FullName:      fullNameFromURL(u),
		FetchURL:      u.String(),
		DefaultBranch: opts.defaultBranch,
		RelPath:       opts.path,
		FetchFlags:    opts.fetchFlags,
		URLs:          []string{u.String()},
	}
}

func fullNameFromURL(u *urlpkg.URL) string {
	return strings.TrimPrefix(strings.TrimSuffix(u.Path, ".git"), "/")
}

func distinctURLs(ss ...string) []string {
	set := make(map[string]struct{})
	for _, s := range ss {
		if s != "" {
			set[s] = struct{}{}
		}
	}
	return slices.Collect(maps.Keys(set))
}
