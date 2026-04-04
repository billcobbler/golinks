package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/billcobbler/golinks/internal/cli"
)

var statsCmd = &cobra.Command{
	Use:   "stats [shortname]",
	Short: "Show usage statistics",
	Long: `Show global usage statistics, or per-link stats for a specific shortname.

Examples:
  golinks stats
  golinks stats docs
  golinks stats --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := mustClient()
		if len(args) == 1 {
			link, err := client.GetLink(args[0])
			if err != nil {
				return err
			}
			cli.PrintLink(os.Stdout, link, jsonOutput)
			return nil
		}
		stats, err := client.GetStats()
		if err != nil {
			return err
		}
		cli.PrintStats(os.Stdout, stats, jsonOutput)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
