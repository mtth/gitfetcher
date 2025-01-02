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
	"github.com/mtth/gitfetcher/internal/except"
	"google.golang.org/protobuf/encoding/prototext"
)

// Config is the generated configuration type, exported here for use in helper signatures.
type Config = configpb.Config

const defaultName = ".gitfetcher.conf"

var (
	errInvalidConfig = errors.New("invalid configuration")
	errMissingConfig = errors.New("configuration not found")
)

func FindConfig(dpath string) (*configpb.Config, error) {
	slog.Debug("Finding config...", slog.String("from", dpath))
	child := dpath
	for {
		info, err := os.Stat(child)
		except.Must(err == nil, "unable to read path %s: %v", child, err)
		except.Must(info.IsDir(), "expected %s to be a directory", child)
		fpath := filepath.Join(child, defaultName)
		cfg, err := readConfig(fpath)
		if err == nil {
			slog.Info("Found config.", slog.String("from", child), slog.String("path", child))
			return cfg, nil
		}
		except.Must(errors.Is(err, errMissingConfig), "error reading config: %v", err)
		parent := filepath.Dir(child)
		if parent == child {
			slog.Info("No config found.", slog.String("from", child))
			var cfg configpb.Config
			ensureRootAbsolute(&cfg, dpath)
			return &cfg, nil
		}
		child = parent
	}
}

func ReadConfig(fpath string) (*configpb.Config, error) {
	slog.Debug("Reading config...", slog.String("path", fpath))
	cfg, err := readConfig(fpath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errMissingConfig
	}
	if err != nil {
		return nil, err
	}
	slog.Info("Read config.", slog.String("path", fpath))
	return cfg, nil
}

var filepathAbs = filepath.Abs

func readConfig(fpath string) (*configpb.Config, error) {
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errMissingConfig, err)
	}
	var cfg configpb.Config
	if err := prototext.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidConfig, err)
	}
	ensureRootAbsolute(&cfg, filepath.Dir(fpath))
	return &cfg, nil
}

func ensureRootAbsolute(cfg *configpb.Config, dpath string) {
	root := cfg.GetOptions().GetRoot()
	if filepath.IsAbs(root) {
		return
	}
	if cfg.GetOptions() == nil {
		cfg.Options = &configpb.Options{}
	}
	base, err := filepathAbs(dpath)
	except.Must(err == nil, "can't make path %v absolute: %v", dpath, err)
	cfg.Options.Root = path.Join(filepath.ToSlash(base), root)
}
