package gitfetcher

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
	"time"

	"github.com/gobwas/glob"
	"github.com/google/go-github/v66/github"
	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
)

// Source captures information about a repository to be mirrored.
type Source struct {
	// Qualified repository name, typically $owner/$name. Non-empty.
	FullName string
	// URL used to fetch repository updates. Non-empty.
	FetchURL string
	// Optional human-readable description. May be empty.
	Description string
	// Default branch. May be empty.
	DefaultBranch string
	// Local relative path override. May be empty.
	RelPath string
	// Last time the remote repository was updated. Zero if unknown.
	LastUpdatedAt time.Time
	// Git flags used to fetch repository updates.
	fetchFlags []string
}

func FullName(u *url.URL) string {
	return strings.TrimPrefix(strings.TrimSuffix(u.Path, ".git"), "/")
}

// LoadSources returns all sources for the provided configuration.
func LoadSources(ctx context.Context, configs []*configpb.Source) ([]Source, error) {
	slog.Debug("Gathering sources...")

	var builder sourcesBuilder
	gatherer := &sourceGatherer{builder: &builder, githubClient: github.NewClient(nil)}
	var errs []error
	for _, config := range configs {
		var err error
		switch b := config.GetBranch().(type) {
		case *configpb.Source_FromUrl:
			err = gatherer.gatherURLSource(ctx, b.FromUrl)
		case *configpb.Source_FromGithubToken:
			err = gatherer.gatherGithubTokenSources(ctx, b.FromGithubToken)
		default:
			return nil, fmt.Errorf("%w: %v", errUnexpectedConfig, config)
		}
		errs = append(errs, err)
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	srcs := builder.build()
	slog.Info(fmt.Sprintf("Found %v source(s).", len(srcs)))
	return srcs, nil
}

type sourcesBuilder []Source

type sourceOptions struct {
	defaultBranch, path string
	fetchFlags          []string
	remoteProtocol      configpb.GithubTokenSource_RemoteProtocol
}

func (b *sourcesBuilder) addStandardURLRepo(u *url.URL, opts sourceOptions) {
	*b = append(*b, Source{
		FullName:      FullName(u),
		FetchURL:      u.String(),
		DefaultBranch: opts.defaultBranch,
		RelPath:       opts.path,
		fetchFlags:    opts.fetchFlags,
	})
}

func (b *sourcesBuilder) addGithubRepo(repo *github.Repository, opts sourceOptions) {
	src := Source{
		FullName:      repo.GetFullName(),
		Description:   repo.GetDescription(),
		DefaultBranch: cmp.Or(opts.defaultBranch, repo.GetDefaultBranch()),
		LastUpdatedAt: repo.GetUpdatedAt().Time,
		RelPath:       opts.path,
		fetchFlags:    opts.fetchFlags,
	}
	switch opts.remoteProtocol {
	case configpb.GithubTokenSource_DEFAULT_REMOTE_PROTOCOL:
		src.FetchURL = repo.GetCloneURL()
	case configpb.GithubTokenSource_SSH_REMOTE_PROTOCOL:
		src.FetchURL = repo.GetSSHURL()
	}
	*b = append(*b, src)
}

func (b *sourcesBuilder) build() []Source {
	return ([]Source)(*b)
}

var (
	errInvalidGithubToken = errors.New("invalid GitHub token")
	errInvalidPath        = errors.New("invalid path")
	errUnexpectedConfig   = errors.New("unexpected config")
	errInvalidURL         = errors.New("invalid URL")
)

// sourceGatherer is a GitHub-backed sourcesGatherer implementation.
type sourceGatherer struct {
	builder      *sourcesBuilder
	githubClient *github.Client
}

func (c *sourceGatherer) gatherURLSource(
	ctx context.Context,
	cfg *configpb.UrlSource,
) error {
	repoURL, err := url.Parse(cfg.GetUrl())
	if err != nil {
		return fmt.Errorf("%w: %s", errInvalidURL, cfg.GetUrl())
	}
	opts := sourceOptions{
		defaultBranch: cfg.GetDefaultBranch(),
		path:          cfg.GetPath(),
	}
	switch repoURL.Hostname() {
	case "github.com":
		folder, name := path.Split(FullName(repoURL))
		repo, _, err := c.githubClient.Repositories.Get(ctx, strings.TrimSuffix(folder, "/"), name)
		if err != nil {
			return fmt.Errorf("unable to get source from URL %v: %w", repoURL, err)
		}
		c.builder.addGithubRepo(repo, opts)
	default:
		c.builder.addStandardURLRepo(repoURL, opts)
	}
	slog.Debug("Added URL source.", dataAttrs(slog.String("url", repoURL.String())))
	return nil
}

func (c *sourceGatherer) gatherGithubTokenSources(
	ctx context.Context,
	cfg *configpb.GithubTokenSource,
) error {
	token := cfg.GetToken()
	if suffix, ok := strings.CutPrefix(token, "$"); ok {
		token = os.Getenv(suffix)
	}

	client := c.githubClient.WithAuthToken(token)
	flags := []string{
		"-c",
		fmt.Sprintf("credential.helper=!f() { echo username=token; echo password=%v; };f", token),
	}

	pred, err := newNamePredicate(cfg.GetFilters())
	if err != nil {
		return err
	}

	opts := &github.RepositoryListByAuthenticatedUserOptions{
		ListOptions: github.ListOptions{PerPage: 50},
	}
	var added, skipped int
	for {
		repos, res, err := client.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err != nil {
			return fmt.Errorf("%w: %w", errInvalidGithubToken, err)
		}
		for _, repo := range repos {
			if (repo.GetFork() && !cfg.GetIncludeForks()) || (repo.GetArchived() && !cfg.GetIncludeArchived()) || !pred.accept(repo.GetFullName()) {
				skipped++
				continue
			}

			path, err := githubSourcePath(cfg.GetPathTemplate(), repo)
			if err != nil {
				return fmt.Errorf("%w: %v", errInvalidPath, err)
			}

			c.builder.addGithubRepo(repo, sourceOptions{
				fetchFlags:     flags,
				remoteProtocol: cfg.GetRemoteProtocol(),
				path:           path,
			})
			added++
		}
		if res.NextPage == 0 {
			break
		}
		opts.Page = res.NextPage
	}
	slog.Debug(
		"Added authenticated source.",
		dataAttrs(slog.Int("added", added), slog.Int("skipped", skipped)),
	)
	return nil
}

func githubSourcePath(tpl string, repo *github.Repository) (string, error) {
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
