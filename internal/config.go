// Package gitfetcher creates local mirrors from remote git repositories.
package gitfetcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config interface {
	SourceProvider() string
}

const defaultName = ".gitfetcher.yaml"

type githubConfig struct {
	Sources []githubSourcesConfig `yaml:"sources"`
}

func (c *githubConfig) SourceProvider() string {
	return "github"
}

type githubSourcesConfig struct {
	Match string `yaml:"match"`
}

type configProduct struct {
	Github *githubConfig `yaml:"github"`
}

var (
	ErrMissingConfig = errors.New("missing configuration")
	ErrInvalidConfig = errors.New("invalid configuration")
)

func ParseConfig(fp string) (Config, error) {
	info, err := os.Stat(fp)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMissingConfig, err)
	}
	if info.IsDir() {
		fp = filepath.Join(fp, defaultName)
	}
	data, err := os.ReadFile(fp)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMissingConfig, err)
	}
	var prod configProduct
	if err := yaml.Unmarshal(data, &prod); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	if prod.Github != nil {
		if len(prod.Github.Sources) == 0 {
			return nil, fmt.Errorf("%w: empty GitHub sources", ErrInvalidConfig)
		}
		return prod.Github, nil
	}
	return nil, fmt.Errorf("%w: empty contents", ErrInvalidConfig)
}
