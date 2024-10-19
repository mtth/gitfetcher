package main

import (
	"cmp"
	"context"
	"fmt"

	gitfetcher "github.com/mtth/gitfetcher/internal"
	"github.com/spf13/cobra"
)

func main() {
	ctx := context.Background()

	var configPath string

	syncCmd := &cobra.Command{
		Use:   "sync PATH",
		Short: "Sync repositories",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root := args[0]
			config, err := gitfetcher.ParseConfig(cmp.Or(configPath, root))
			if err != nil {
				return err
			}
			srcs, err := gitfetcher.FindSources(ctx, config)
			if err != nil {
				return err
			}
			return gitfetcher.SyncSources(ctx, root, srcs)
		},
	}

	showCmd := &cobra.Command{
		Use:   "show [PATH]",
		Short: "Show repositories",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root := args[0]
			config, err := gitfetcher.ParseConfig(cmp.Or(configPath, root))
			if err != nil {
				return err
			}
			srcs, err := gitfetcher.FindSources(ctx, config)
			if err != nil {
				return err
			}
			for _, src := range srcs {
				fmt.Printf("%s\t%s\n", src.Name, src.FetchURL)
			}
			return nil
		},
	}

	rootCmd := &cobra.Command{Use: "gitfetcher"}
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to configuration file.")
	rootCmd.AddCommand(syncCmd, showCmd)

	_ = rootCmd.ExecuteContext(ctx)
}
