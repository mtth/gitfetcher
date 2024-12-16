// Package gitfetcher creates local mirrors from remote git repositories.
package gitfetcher

//go:generate mkdir -p configpb_gen
//go:generate protoc --proto_path=../api ../api/config.proto --go_out=configpb_gen --go_opt=paths=source_relative

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	configpb "github.com/mtth/gitfetcher/internal/configpb_gen"
	"google.golang.org/protobuf/encoding/prototext"
)

// Config is the generated configuration type, exported here for use in helper signatures.
type Config = configpb.Config

const defaultName = ".gitfetcher.conf"

var (
	errMissingConfig = errors.New("missing configuration")
	errInvalidConfig = errors.New("invalid configuration")
)

var filepathAbs = filepath.Abs

// ParseConfig returns a parsed configuration from a given path. The path may either point to a
// configuration file or a folder, in which case the default configuration file name will be used.
// The configuration's root will be automatically populated.
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

	root := cfg.GetOptions().GetRoot()
	if !filepath.IsAbs(root) {
		if cfg.GetOptions() == nil {
			cfg.Options = &configpb.Options{}
		}
		base, err := filepathAbs(filepath.Dir(fp))
		if err != nil {
			return nil, err
		}
		cfg.Options.Root = path.Join(filepath.ToSlash(base), root)
	}

	slog.Info("Read config.", dataAttrs(slog.String("path", fp), slog.String("root", root)))
	return &cfg, nil
}
