// Package gitfetcher creates local mirrors from remote git repositories.
package gitfetcher

//go:generate mkdir -p configpb_gen
//go:generate protoc --proto_path=../api ../api/config.proto --go_out=configpb_gen --go_opt=paths=source_relative

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"google.golang.org/protobuf/encoding/prototext"
)

const defaultName = ".gitfetcher"

var (
	errMissingConfig = errors.New("missing configuration")
	errInvalidConfig = errors.New("invalid configuration")
)

// ParseConfig returns a parsed configuration from a given path. The path may either point to a
// configuration file or a folder, in which case the default configuration file name will be used.
func ParseConfig(fp string) (*configpb.Config, error) {
	slog.Debug("Reading config...", dataAttrs(slog.String("path", fp)))

	info, err := os.Stat(fp)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errMissingConfig, err)
	}
	if info.IsDir() {
		fp = filepath.Join(fp, defaultName)
	}
	data, err := os.ReadFile(fp)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errMissingConfig, err)
	}
	var cfg configpb.Config
	if err := prototext.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidConfig, err)
	}
	if cfg.GetBranch() == nil {
		return nil, fmt.Errorf("%w: empty contents", errInvalidConfig)
	}

	slog.Info("Read config.", dataAttrs(slog.String("path", fp)))
	return &cfg, nil
}
