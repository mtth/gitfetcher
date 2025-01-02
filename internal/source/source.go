package source

import (
	"cmp"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"
	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"github.com/mtth/gitfetcher/internal/fspath"
)

// Concepts:
//
// * Config source => SourceConfig
// * Local repository => Target
// * Source metadata => Source
//
// Operations:
//
// * Compare repo with source
// * Fetch source metadata (batch)
// * Update repo

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

type sourcesBuilder []Source

type sourceOptions struct {
	defaultBranch  string
	path           fspath.POSIX
	fetchFlags     []string
	remoteProtocol configpb.RemoteProtocol
}

func (b *sourcesBuilder) addStandardURLRepo(u *url.URL, opts sourceOptions) {
	*b = append(*b, Source{
		FullName:      fullNameFromURL(u),
		FetchURL:      u.String(),
		DefaultBranch: opts.defaultBranch,
		RelPath:       opts.path,
		FetchFlags:    opts.fetchFlags,
	})
}

func (b *sourcesBuilder) addGithubRepo(repo *github.Repository, opts sourceOptions) {
	src := Source{
		FullName:      repo.GetFullName(),
		Description:   repo.GetDescription(),
		DefaultBranch: cmp.Or(opts.defaultBranch, repo.GetDefaultBranch()),
		LastUpdatedAt: repo.GetUpdatedAt().Time,
		RelPath:       opts.path,
		FetchFlags:    opts.fetchFlags,
	}
	switch opts.remoteProtocol {
	case configpb.RemoteProtocol_DEFAULT_REMOTE_PROTOCOL:
		src.FetchURL = repo.GetCloneURL()
	case configpb.RemoteProtocol_SSH_REMOTE_PROTOCOL:
		src.FetchURL = repo.GetSSHURL()
	}
	*b = append(*b, src)
}

func (b *sourcesBuilder) build() []Source {
	return ([]Source)(*b)
}

func fullNameFromURL(u *url.URL) string {
	return strings.TrimPrefix(strings.TrimSuffix(u.Path, ".git"), "/")
}
