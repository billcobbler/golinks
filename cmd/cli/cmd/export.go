package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportOutput string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all golinks to a file",
	Example: `  golinks export                          # JSON to stdout
  golinks export --format csv -o links.csv
  golinks export --format json -o backup.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := mustClient().Export(exportFormat)
		if err != nil {
			return err
		}
		if exportOutput == "" || exportOutput == "-" {
			_, err = os.Stdout.Write(data)
			return err
		}
		if err := os.WriteFile(exportOutput, data, 0o644); err != nil {
			return fmt.Errorf("writing file: %w", err)
		}
		fmt.Printf("Exported %d bytes to %s\n", len(data), exportOutput)
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json", "export format: json or csv")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "output file path (default: stdout)")
	rootCmd.AddCommand(exportCmd)
}
