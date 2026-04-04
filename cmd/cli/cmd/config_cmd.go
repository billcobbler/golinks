package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/billcobbler/golinks/internal/cli"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage golinks CLI configuration",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value. Valid keys:
  server   The golinks server URL (default: http://localhost:8080)
  token    API token for authentication`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]
		if key != "server" && key != "token" {
			return fmt.Errorf("unknown config key %q (valid keys: server, token)", key)
		}
		if err := cli.SetConfigValue(key, value); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Print a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		val, err := cli.GetConfigValue(args[0])
		if err != nil {
			return err
		}
		fmt.Println(val)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show all configuration values",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cli.LoadConfig()
		if err != nil {
			return err
		}
		dir, _ := cli.ConfigDir()
		fmt.Printf("server: %s\n", cfg.Server)
		if cfg.Token != "" {
			fmt.Printf("token:  %s\n", cfg.Token)
		} else {
			fmt.Println("token:  (not set)")
		}
		fmt.Printf("file:   %s/config.yaml\n", dir)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}
