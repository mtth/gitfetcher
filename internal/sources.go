package gitfetcher

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"github.com/google/go-github/v66/github"
	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
)

// Source captures information about a repository to be mirrored.
type Source struct {
	Name, FetchURL, Description, DefaultBranch, Path string
	LastUpdatedAt                                    time.Time
	fetchFlags                                       []string
}

// FindSources returns all sources for the provided configuration.
func FindSources(ctx context.Context, cfg *configpb.Config) ([]*Source, error) {
	slog.Debug("Finding sources...")

	var builder sourcesBuilder
	finder := &sourceFinder{builder: &builder, githubClient: github.NewClient(nil)}

	var errs []error
	for _, src := range cfg.GetSources() {
		var err error
		switch b := src.GetBranch().(type) {
		case *configpb.Source_FromUrl:
			err = finder.findURLSource(ctx, b.FromUrl)
		case *configpb.Source_FromGithubToken:
			err = finder.findGithubTokenSources(ctx, b.FromGithubToken)
		default:
			return nil, fmt.Errorf("%w: %v", errUnexpectedConfig, cfg)
		}
		errs = append(errs, err)
	}

	srcs := builder.build()
	err := errors.Join(errs...)
	slog.Info(fmt.Sprintf("Found %v source(s).", len(srcs)), errAttr(err))
	return srcs, err
}

type sourcesBuilder []*Source

type sourceOptions struct {
	defaultBranch, path string
	fetchFlags          []string
	remoteProtocol      configpb.GithubTokenSource_RemoteProtocol
}

const defaultBranch = "main"

func (b *sourcesBuilder) addStandardURLRepo(url string, name string, opts sourceOptions) {
	*b = append(*b, &Source{
		Name:          name,
		FetchURL:      url,
		DefaultBranch: cmp.Or(opts.defaultBranch, defaultBranch),
		Path:          opts.path,
		fetchFlags:    opts.fetchFlags,
	})
}

func (b *sourcesBuilder) addGithubRepo(repo *github.Repository, opts sourceOptions) {
	src := &Source{
		Name:          repo.GetFullName(),
		Description:   repo.GetDescription(),
		DefaultBranch: cmp.Or(opts.defaultBranch, repo.GetDefaultBranch(), defaultBranch),
		LastUpdatedAt: repo.GetUpdatedAt().Time,
		Path:          opts.path,
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

func (b *sourcesBuilder) build() []*Source {
	return ([]*Source)(*b)
}

var (
	errInvalidGithubToken = errors.New("invalid GitHub token")
	errUnexpectedConfig   = errors.New("unexpected config")
	errUnsupportedURL     = errors.New("unsupported URL")
)

// sourceFinder is a GitHub-backed sourcesFinder implementation.
type sourceFinder struct {
	builder      *sourcesBuilder
	githubClient *github.Client
}

var standardURLPattern = regexp.MustCompile(`^https://([^/]+)/([^/]+)/([^/]+)/?$`)

func (c *sourceFinder) findURLSource(
	ctx context.Context,
	cfg *configpb.UrlSource,
) error {
	url := cfg.GetUrl()
	matches := standardURLPattern.FindStringSubmatch(url)
	if matches == nil {
		return fmt.Errorf("%w: %s", errUnsupportedURL, url)
	}
	opts := sourceOptions{
		defaultBranch: cfg.GetDefaultBranch(),
		path:          cfg.GetPath(),
	}
	suffix := strings.TrimSuffix(matches[3], ".git")
	switch matches[1] {
	case "github.com":
		repo, _, err := c.githubClient.Repositories.Get(ctx, matches[2], suffix)
		if err != nil {
			return fmt.Errorf("unable to get source from URL %s: %w", url, err)
		}
		c.builder.addGithubRepo(repo, opts)
	default:
		c.builder.addStandardURLRepo(url, fmt.Sprintf("%s/%s", matches[2], suffix), opts)
	}
	slog.Debug("Added URL source.", dataAttrs(slog.String("url", url)))
	return nil
}

func (c *sourceFinder) findGithubTokenSources(
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
			if (repo.GetFork() && !cfg.GetIncludeForks()) || !pred.accept(repo.GetFullName()) {
				skipped++
				continue
			}
			c.builder.addGithubRepo(repo, sourceOptions{
				fetchFlags:     flags,
				remoteProtocol: cfg.GetRemoteProtocol(),
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
