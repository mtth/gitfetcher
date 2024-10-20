package gitfetcher

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"github.com/google/go-github/v66/github"
	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
)

// Source captures information about a repository to be mirrored.
type Source struct {
	Name, FetchURL, Description, defaultBranch string
	fetchFlags                                 []string
	LastUpdatedAt                              time.Time
}

// FindSources returns all sources for the provided configuration.
func FindSources(ctx context.Context, cfg *configpb.Config) ([]*Source, error) {
	slog.Debug("Finding sources...")

	var finder sourcesFinder
	switch b := cfg.GetBranch().(type) {
	case *configpb.Config_Github:
		finder = &githubSourceFinder{client: github.NewClient(nil), config: b.Github}
	default:
		return nil, fmt.Errorf("%w: %v", errUnexpectedConfig, cfg)
	}

	srcs, err := finder.findSources(ctx)
	slog.Info(fmt.Sprintf("Found %v source(s).", len(srcs)), errAttr(err))
	return srcs, err
}

// sourcesFinder retrieves Source metadata.
type sourcesFinder interface {
	findSources(ctx context.Context) ([]*Source, error)
}

var (
	errInvalidGithubToken = errors.New("invalid GitHub token")
	errUnexpectedConfig   = errors.New("unexpected config")
)

// githubSourceFinder is a GitHub-backed sourcesFinder implementation.
type githubSourceFinder struct {
	client *github.Client
	config *configpb.Github
}

type githubSourcesBuilder []*Source

func (b *githubSourcesBuilder) add(repo *github.Repository, flags []string) {
	*b = append(*b, &Source{
		Name:          repo.GetFullName(),
		FetchURL:      repo.GetCloneURL(),
		Description:   repo.GetDescription(),
		fetchFlags:    flags,
		defaultBranch: repo.GetDefaultBranch(),
		LastUpdatedAt: repo.GetUpdatedAt().Time,
	})
}

func (b *githubSourcesBuilder) build() []*Source {
	return ([]*Source)(*b)
}

// findSources implements sourcesFinder.
func (c *githubSourceFinder) findSources(ctx context.Context) ([]*Source, error) {
	var errs []error
	var builder githubSourcesBuilder
	for _, cfg := range c.config.Sources {
		switch b := cfg.GetBranch().(type) {
		case *configpb.GithubSource_Name:
			if err := c.addSourceNamed(ctx, &builder, b.Name); err != nil {
				errs = append(errs, err)
			}
		case *configpb.GithubSource_Auth:
			if err := c.addAuthenticatedSources(ctx, &builder, b.Auth); err != nil {
				errs = append(errs, err)
			}
		default:
			return nil, errUnexpectedConfig
		}
	}
	return builder.build(), errors.Join(errs...)
}

func (c *githubSourceFinder) addSourceNamed(
	ctx context.Context,
	builder *githubSourcesBuilder,
	name string,
) error {
	parts := strings.SplitN(name, "/", 2)
	repo, _, err := c.client.Repositories.Get(ctx, parts[0], parts[1])
	if err != nil {
		return fmt.Errorf("unable to get source named %s: %w", name, err)
	}
	builder.add(repo, nil)
	slog.Debug("Added named source.", dataAttrs(slog.String("name", name)))
	return nil
}

func (c *githubSourceFinder) addAuthenticatedSources(
	ctx context.Context,
	builder *githubSourcesBuilder,
	auth *configpb.GithubSource_TokenAuth,
) error {
	token := auth.GetToken()
	if suffix, ok := strings.CutPrefix(token, "$"); ok {
		token = os.Getenv(suffix)
	}

	client := c.client.WithAuthToken(token)
	flags := []string{
		"-c",
		fmt.Sprintf("credential.helper=!f() { echo username=token; echo password=%v; };f", token),
	}

	pred, err := newNamePredicate(auth.GetFilters())
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
			if (repo.GetFork() && !auth.GetIncludeForks()) || !pred.accept(repo.GetName()) {
				skipped++
				continue
			}
			builder.add(repo, flags)
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
