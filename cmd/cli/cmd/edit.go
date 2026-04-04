package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/billcobbler/golinks/internal/cli"
)

var (
	editURL         string
	editDescription string
	editPattern     bool
)

var editCmd = &cobra.Command{
	Use:   "edit <shortname>",
	Short: "Update an existing golink",
	Example: `  golinks edit docs --url https://newdocs.example.com
  golinks edit docs --description "Main documentation"
  golinks edit gh/repo --pattern`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := mustClient()
		shortname := args[0]

		// Fetch existing link so we can merge unchanged fields (PUT replaces all).
		existing, err := client.GetLink(shortname)
		if err != nil {
			return err
		}

		input := cli.UpdateLinkInput{
			TargetURL:   existing.TargetURL,
			Description: existing.Description,
			IsPattern:   existing.IsPattern,
		}
		if cmd.Flags().Changed("url") {
			input.TargetURL = editURL
		}
		if cmd.Flags().Changed("description") {
			input.Description = editDescription
		}
		if cmd.Flags().Changed("pattern") {
			input.IsPattern = editPattern
		}

		link, err := client.UpdateLink(shortname, input)
		if err != nil {
			return err
		}
		if jsonOutput {
			cli.PrintLink(os.Stdout, link, true)
		} else {
			fmt.Printf("Updated: go/%s → %s\n", link.Shortname, link.TargetURL)
		}
		return nil
	},
}

func init() {
	editCmd.Flags().StringVarP(&editURL, "url", "u", "", "new target URL")
	editCmd.Flags().StringVarP(&editDescription, "description", "d", "", "new description")
	editCmd.Flags().BoolVar(&editPattern, "pattern", false, "enable/disable pattern mode")
	rootCmd.AddCommand(editCmd)
}
