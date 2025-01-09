// Package source loads remote information about repositories.
package source

import (
	"net/url"
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
	// URL used to fetch repository updates. Non-empty.
	FetchURL string
	// Git flags used to fetch repository updates.
	FetchFlags []string
}

type options struct {
	defaultBranch  string
	path           fspath.POSIX
	fetchFlags     []string
	remoteProtocol configpb.RemoteProtocol
}

func fromURL(u *url.URL, opts options) *Source {
	return &Source{
		FullName:      fullNameFromURL(u),
		FetchURL:      u.String(),
		DefaultBranch: opts.defaultBranch,
		RelPath:       opts.path,
		FetchFlags:    opts.fetchFlags,
	}
}

func fullNameFromURL(u *url.URL) string {
	return strings.TrimPrefix(strings.TrimSuffix(u.Path, ".git"), "/")
}
