package gitfetcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/go-github/v66/github"
)

// source captures information about a repository to be mirrored.
type source struct {
	name, fetchURL, description, defaultBranch string
	fetchFlags                                 []string
	lastUpdatedAt                              time.Time
}

// sourceFinder retrieves source metadata.
type sourceFinder interface {
	findSources(ctx context.Context) ([]*source, error)
}

var getenv = os.Getenv

var (
	errMissingGithubToken = errors.New("missing or empty GITHUB_TOKEN environment variable")
	errInvalidGithubToken = errors.New("invalid GitHub token")
	errUnexpectedConfig   = errors.New("unexpected config")
)

// newSourceFinder creates a sourceFinder for the provided configuration.
func newSourceFinder(cfg Config) (sourceFinder, error) {
	switch c := cfg.(type) {
	case *githubConfig:
		token := getenv("GITHUB_TOKEN")
		if token == "" {
			return nil, errMissingGithubToken
		}
		return &githubSourceFinder{
			config: c,
			github: github.NewClient(nil).WithAuthToken(token),
			token:  token,
		}, nil
	default:
		return nil, fmt.Errorf("%w: %v", errUnexpectedConfig, cfg)
	}
}

// githubSourceFinder is a GitHub-backed sourceFinder implementation.
type githubSourceFinder struct {
	config *githubConfig
	github *github.Client
	token  string
}

// findSources implements sourceFinder.
func (c *githubSourceFinder) findSources(ctx context.Context) ([]*source, error) {
	flags := []string{
		"-c",
		fmt.Sprintf("credential.helper=!f() { echo username=token; echo password=%v; };f", c.token),
	}

	var srcs []*source
	opts := &github.RepositoryListByAuthenticatedUserOptions{
		ListOptions: github.ListOptions{PerPage: 50},
	}
	for {
		repos, res, err := c.github.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidGithubToken, err)
		}
		for _, repo := range repos {
			if repo.GetFork() {
				continue
			}
			srcs = append(srcs, &source{
				name:          repo.GetFullName(),
				fetchURL:      repo.GetCloneURL(),
				fetchFlags:    flags,
				defaultBranch: repo.GetDefaultBranch(),
				description:   repo.GetDescription(),
				lastUpdatedAt: repo.GetUpdatedAt().Time,
			})
		}
		if res.NextPage == 0 {
			break
		}
		opts.Page = res.NextPage
	}

	return srcs, nil
}
