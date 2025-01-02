// Package main implements the gitfetcher CLI.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/adrg/xdg"
	gitfetcher "github.com/mtth/gitfetcher/internal"
	"github.com/mtth/gitfetcher/internal/except"
	"github.com/spf13/cobra"
)

func init() {
	var errs []error

	fp, ok := os.LookupEnv("LOGS_DIRECTORY")
	if !ok {
		var err error
		fp, err = xdg.StateFile("gitfetcher/log")
		if err != nil {
			errs = append(errs, err)
			fp = "gitfetcher.log"
		}
	}

	var writer io.Writer
	if file, err := os.OpenFile(fp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		writer = file
	} else {
		errs = append(errs, err)
		writer = os.Stdout
	}

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))
	if len(errs) > 0 {
		slog.Error("Log setup failed.", except.LogErrAttr(errors.Join(errs...)))
	}
}

var (
	configPath string
)

func main() {
	ctx := context.Background()

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync repositories",
		Args:  cobra.MaximumNArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			syncables, err := gatherSyncables(ctx)
			if err != nil {
				return err
			}
			for _, syncable := range syncables {
				if err := syncable.Sync(ctx); err != nil {
					return err
				}
			}
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show repository statuses",
		Args:  cobra.MaximumNArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			syncables, err := gatherSyncables(ctx)
			if err != nil {
				return err
			}
			for _, syncable := range syncables {
				status := syncable.SyncStatus()
				fmt.Printf("%v\t%s\n", status, syncable.Path) //nolint:forbidigo
			}
			return nil
		},
	}

	rootCmd := &cobra.Command{Use: "gitfetcher", SilenceUsage: true}
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "path to configuration")
	rootCmd.AddCommand(syncCmd, statusCmd)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

func gatherSyncables(ctx context.Context) ([]gitfetcher.Syncable, error) {
	config, err := loadConfig()
	if err != nil {
		return nil, err
	}
	root := config.GetOptions().GetRoot()
	targets, err := gitfetcher.FindTargets(root)
	if err != nil {
		return nil, err
	}
	sources, err := gitfetcher.LoadSources(ctx, config.GetSources())
	if err != nil {
		return nil, err
	}
	return gitfetcher.GatherSyncables(targets, sources, root, config.GetOptions().GetInitLayout())
}

func loadConfig() (*gitfetcher.Config, error) {
	if configPath != "" {
		return gitfetcher.ReadConfig(configPath)
	}
	return gitfetcher.FindConfig(".")
}
