package source

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
)

var newGithubClient = newRealGithubClient

// Load returns all sources for the provided configuration.
func Load(ctx context.Context, origins []*configpb.Origin) ([]*Source, error) {
	slog.Debug("Loading sources...")

	l := &loader{githubClient: newGithubClient()}
	var errs []error
	for _, origin := range origins {
		var err error
		switch b := origin.GetBranch().(type) {
		case *configpb.Origin_FromUrl:
			err = l.loadURLSource(ctx, b.FromUrl)
		case *configpb.Origin_FromGithubToken:
			err = l.loadGithubTokenSources(ctx, b.FromGithubToken)
		default:
			return nil, fmt.Errorf("%w: %v", errUnexpectedOrigin, origin)
		}
		errs = append(errs, err)
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	slog.Info(fmt.Sprintf("Loaded %v source(s).", len(l.sources)))
	return l.sources, nil
}

var (
	errInvalidPath      = errors.New("invalid path")
	errUnexpectedOrigin = errors.New("unexpected origin")
	errInvalidURL       = errors.New("invalid URL")
)

type loader struct {
	sources      []*Source
	githubClient githubClient
}

func (c *loader) loadURLSource(
	ctx context.Context,
	origin *configpb.UrlOrigin,
) error {
	u, err := url.Parse(origin.GetUrl())
	if err != nil {
		return fmt.Errorf("%w: %s", errInvalidURL, origin.GetUrl())
	}

	switch u.Hostname() {
	case "github.com":
		src, err := c.githubClient.urlSource(ctx, origin)
		if err != nil {
			return err
		}
		c.sources = append(c.sources, src)
	default:
		c.sources = append(c.sources, fromURL(u, options{
			defaultBranch: origin.GetDefaultBranch(),
			path:          origin.GetPath(),
		}))
	}

	slog.Debug("Added URL source.", slog.String("url", u.String()))
	return nil
}

func (c *loader) loadGithubTokenSources(
	ctx context.Context,
	origin *configpb.GithubTokenOrigin,
) error {
	srcs, err := c.githubClient.tokenSources(ctx, origin)
	if err != nil {
		return err
	}
	c.sources = append(c.sources, srcs...)
	return nil
}
