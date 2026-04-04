package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/billcobbler/golinks/internal/cli"
)

var (
	addDescription string
	addPattern     bool
)

var addCmd = &cobra.Command{
	Use:   "add <shortname> <url>",
	Short: "Create a new golink",
	Example: `  golinks add docs https://docs.example.com
  golinks add gh/myrepo https://github.com/org/{*} --pattern
  golinks add hr/benefits https://hr.example.com/benefits -d "HR benefits portal"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		link, err := mustClient().CreateLink(cli.CreateLinkInput{
			Shortname:   args[0],
			TargetURL:   args[1],
			Description: addDescription,
			IsPattern:   addPattern,
		})
		if err != nil {
			return err
		}
		if jsonOutput {
			cli.PrintLink(os.Stdout, link, true)
		} else {
			fmt.Printf("Created: go/%s → %s\n", link.Shortname, link.TargetURL)
		}
		return nil
	},
}

func init() {
	addCmd.Flags().StringVarP(&addDescription, "description", "d", "", "description for this link")
	addCmd.Flags().BoolVar(&addPattern, "pattern", false, "treat as a pattern link ({*}, {1}, {2} substitution in URL)")
	rootCmd.AddCommand(addCmd)
}
