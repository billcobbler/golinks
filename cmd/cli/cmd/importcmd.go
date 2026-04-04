package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var importOverwrite bool

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import golinks from a JSON or CSV file",
	Example: `  golinks import links.json
  golinks import links.csv --overwrite`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
		contentType := "application/json"
		if strings.HasSuffix(strings.ToLower(filePath), ".csv") {
			contentType = "text/csv"
		}
		msg, err := mustClient().Import(data, contentType, importOverwrite)
		if err != nil {
			return err
		}
		fmt.Println(msg)
		return nil
	},
}

func init() {
	importCmd.Flags().BoolVar(&importOverwrite, "overwrite", false, "overwrite existing links with the same shortname")
	rootCmd.AddCommand(importCmd)
}
