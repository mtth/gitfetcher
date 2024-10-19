// Package gitfetcher creates local mirrors from remote git repositories.
package gitfetcher

//go:generate mkdir -p configpb_gen
//go:generate protoc --proto_path=../api ../api/config.proto --go_out=configpb_gen --go_opt=paths=source_relative

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/encoding/prototext"

	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
)

const defaultName = ".gitfetcher"

var (
	ErrMissingConfig = errors.New("missing configuration")
	ErrInvalidConfig = errors.New("invalid configuration")
)

func ParseConfig(fp string) (*configpb.Config, error) {
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
	var cfg configpb.Config
	if err := prototext.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	if cfg.GetBranch() == nil {
		return nil, fmt.Errorf("%w: empty contents", ErrInvalidConfig)
	}
	return &cfg, nil
}
