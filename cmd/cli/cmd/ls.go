package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/billcobbler/golinks/internal/cli"
)

var (
	lsSearch string
	lsLimit  int
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List golinks",
	Example: `  golinks ls
  golinks ls --search github
  golinks ls --limit 100 --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := mustClient().ListLinks(lsSearch, 0, lsLimit)
		if err != nil {
			return err
		}
		if !jsonOutput && len(result.Links) == 0 {
			fmt.Println("No links found.")
			return nil
		}
		cli.PrintLinks(os.Stdout, result.Links, jsonOutput)
		if !jsonOutput && result.Total > len(result.Links) {
			fmt.Fprintf(os.Stderr, "\nShowing %d of %d links. Use --limit to see more.\n",
				len(result.Links), result.Total)
		}
		return nil
	},
}

func init() {
	lsCmd.Flags().StringVarP(&lsSearch, "search", "s", "", "filter by search query (matches shortname, URL, description)")
	lsCmd.Flags().IntVarP(&lsLimit, "limit", "l", 50, "maximum number of links to return")
	rootCmd.AddCommand(lsCmd)
}
