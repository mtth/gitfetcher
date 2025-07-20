package source

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/gobwas/glob"
	"github.com/google/go-github/v66/github"
	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"github.com/mtth/gitfetcher/internal/except"
	"github.com/mtth/gitfetcher/internal/fspath"
)

type githubClient interface {
	urlSource(context.Context, *configpb.UrlOrigin) (*Source, error)
	tokenSources(context.Context, *configpb.GithubTokenOrigin) ([]*Source, error)
}

func newRealGithubClient() githubClient {
	return realGithubClient{github.NewClient(nil)}
}

type realGithubClient struct {
	client *github.Client
}

var (
	errInvalidGithubToken = errors.New("invalid GitHub token")
)

func (c realGithubClient) urlSource(ctx context.Context, origin *configpb.UrlOrigin) (*Source, error) {
	u, err := url.Parse(origin.GetUrl())
	except.Must(err == nil, "invalid URL: %v", err)
	opts := options{
		defaultBranch: origin.GetDefaultBranch(),
		path:          origin.GetPath(),
	}
	folder, name := path.Split(fullNameFromURL(u))
	repo, _, err := c.client.Repositories.Get(ctx, strings.TrimSuffix(folder, "/"), name)
	if err != nil {
		return nil, fmt.Errorf("unable to get source from URL %v: %w", u, err)
	}
	return fromGithubRepo(repo, opts), nil
}

func (c realGithubClient) tokenSources(ctx context.Context, origin *configpb.GithubTokenOrigin) ([]*Source, error) {
	token := origin.GetToken()
	if suffix, ok := strings.CutPrefix(token, "$"); ok {
		token = os.Getenv(suffix)
	}

	client := c.client.WithAuthToken(token)
	flags := []string{
		"-c",
		fmt.Sprintf("credential.helper=!f() { echo username=token; echo password=%v; };f", token),
	}

	pred, err := newNamePredicate(origin.GetFilters())
	if err != nil {
		return nil, err
	}

	opts := &github.RepositoryListByAuthenticatedUserOptions{
		ListOptions: github.ListOptions{PerPage: 50},
	}
	var srcs []*Source
	var added, skipped int
	for {
		repos, res, err := client.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidGithubToken, err)
		}
		for _, repo := range repos {
			if (repo.GetFork() && !origin.GetIncludeForks()) ||
				(repo.GetArchived() && !origin.GetIncludeArchived()) ||
				!pred.accept(repo.GetFullName()) {
				skipped++
				continue
			}

			path, err := githubSourcePath(origin.GetPathTemplate(), repo)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", errInvalidPath, err)
			}

			srcs = append(srcs, fromGithubRepo(repo, options{
				fetchFlags:     flags,
				remoteProtocol: origin.GetRemoteProtocol(),
				path:           path,
			}))
			added++
		}
		if res.NextPage == 0 {
			break
		}
		opts.Page = res.NextPage
	}

	slog.Debug("Added sources from GitHub token.", slog.Int("added", added), slog.Int("skipped", skipped))
	return srcs, nil
}

func fromGithubRepo(repo *github.Repository, opts options) *Source {
	src := &Source{
		FullName:      repo.GetFullName(),
		Description:   repo.GetDescription(),
		DefaultBranch: cmp.Or(opts.defaultBranch, repo.GetDefaultBranch()),
		LastUpdatedAt: repo.GetUpdatedAt().Time,
		RelPath:       opts.path,
		FetchFlags:    opts.fetchFlags,
		URLs:          distinctURLs(repo.GetCloneURL(), repo.GetGitURL(), repo.GetSSHURL()),
	}
	switch opts.remoteProtocol {
	case configpb.RemoteProtocol_DEFAULT_REMOTE_PROTOCOL:
		src.FetchURL = repo.GetCloneURL()
	case configpb.RemoteProtocol_SSH_REMOTE_PROTOCOL:
		src.FetchURL = repo.GetSSHURL()
	}
	return src
}

type namePredicate []glob.Glob

func newNamePredicate(pats []string) (namePredicate, error) {
	var globs []glob.Glob
	for _, filter := range pats {
		compiled, err := glob.Compile(filter)
		if err != nil {
			return nil, err
		}
		globs = append(globs, compiled)
	}
	return namePredicate(globs), nil
}

func (p namePredicate) accept(name string) bool {
	if len(p) == 0 {
		return true
	}
	for _, g := range p {
		if g.Match(name) {
			return true
		}
	}
	return false
}

func githubSourcePath(tpl string, repo *github.Repository) (fspath.POSIX, error) {
	if tpl == "" {
		return "", nil
	}
	parsed, err := template.New("path").Parse(tpl)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	err = parsed.Execute(&b, map[string]string{
		"FullName": repo.GetFullName(),
		"Name":     repo.GetName(),
		"Owner":    repo.GetOwner().GetName(),
	})
	return b.String(), err
}
