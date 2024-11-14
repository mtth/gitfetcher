// Package main implements the gitfetcher CLI.
package main

import (
	"context"
	"fmt"

	gitfetcher "github.com/mtth/gitfetcher/internal"
	"github.com/spf13/cobra"
)

func main() {
	ctx := context.Background()

	syncCmd := &cobra.Command{
		Use:   "sync [PATH]",
		Short: "Sync repositories",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := loadConfig(args)
			if err != nil {
				return err
			}
			srcs, err := gitfetcher.FindSources(ctx, cfg)
			if err != nil {
				return err
			}
			return gitfetcher.Sync(ctx, srcs, cfg.GetOptions())
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status [PATH]",
		Short: "Show repository statuses",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := loadConfig(args)
			if err != nil {
				return err
			}
			srcs, err := gitfetcher.FindSources(ctx, cfg)
			if err != nil {
				return err
			}
			for _, src := range srcs {
				status := gitfetcher.GetSyncStatus(src, cfg.GetOptions())
				fmt.Printf("%v\t%s\t%s\n", status, src.Name, src.FetchURL) //nolint:forbidigo
			}
			return nil
		},
	}

	rootCmd := &cobra.Command{Use: "gitfetcher", SilenceUsage: true}
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.AddCommand(syncCmd, statusCmd)

	_ = rootCmd.ExecuteContext(ctx)
}

func loadConfig(args []string) (*gitfetcher.Config, error) {
	fp := "."
	if len(args) > 0 {
		fp = args[0]
	}
	return gitfetcher.ParseConfig(fp)
}
