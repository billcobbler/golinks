package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/billcobbler/golinks/internal/cli"
)

var infoCmd = &cobra.Command{
	Use:   "info <shortname>",
	Short: "Show details for a golink",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		link, err := mustClient().GetLink(args[0])
		if err != nil {
			return err
		}
		cli.PrintLink(os.Stdout, link, jsonOutput)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
