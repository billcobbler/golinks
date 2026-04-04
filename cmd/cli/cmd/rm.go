package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:     "rm <shortname>",
	Aliases: []string{"delete", "del"},
	Short:   "Delete a golink",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := mustClient().DeleteLink(args[0]); err != nil {
			return err
		}
		fmt.Printf("Deleted: go/%s\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(rmCmd)
}
