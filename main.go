package main

import (
	"cmp"
	"context"

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
		RunE: func(cmd *cobra.Command, args []string) error {
			root := args[0]
			config, err := gitfetcher.ParseConfig(cmp.Or(configPath, root))
			if err != nil {
				return err
			}
			fetcher, err := gitfetcher.NewSourceFetcher(root, config)
			if err != nil {
				return err
			}
			return fetcher.FetchSources(ctx)
		},
	}

	// TODO: Add show command to output status of each source (missing, up-to-date, stale).

	rootCmd := &cobra.Command{Use: "gitfetcher"}
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file.")
	rootCmd.AddCommand(syncCmd)
	rootCmd.Execute()
}
