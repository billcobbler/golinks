package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/billcobbler/golinks/internal/cli"
)

var (
	jsonOutput bool
	serverFlag string
	tokenFlag  string
)

var rootCmd = &cobra.Command{
	Use:   "golinks",
	Short: "Manage golinks from the terminal",
	Long: `golinks is a CLI for managing short links on your golinks server.

Examples:
  golinks add docs https://docs.example.com
  golinks ls --search github
  golinks open docs
  golinks config set server http://localhost:8080`,
}

// Execute runs the root command. Called by main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// mustClient loads config and returns an API client, applying any flag overrides.
// Exits with an error message if config cannot be loaded.
func mustClient() *cli.Client {
	cfg, err := cli.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if serverFlag != "" {
		cfg.Server = serverFlag
	}
	if tokenFlag != "" {
		cfg.Token = tokenFlag
	}
	return cli.NewClient(cfg)
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.PersistentFlags().StringVar(&serverFlag, "server", "", "golinks server URL (overrides config)")
	rootCmd.PersistentFlags().StringVar(&tokenFlag, "token", "", "API token (overrides config)")
}
