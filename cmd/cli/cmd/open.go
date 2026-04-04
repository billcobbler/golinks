package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open <shortname>",
	Short: "Open a golink in the default browser",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		link, err := mustClient().GetLink(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Opening %s\n", link.TargetURL)
		return openBrowser(link.TargetURL)
	},
}

func openBrowser(url string) error {
	var browserCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		browserCmd = exec.Command("open", url)
	case "windows":
		browserCmd = exec.Command("cmd", "/c", "start", url)
	default:
		browserCmd = exec.Command("xdg-open", url)
	}
	return browserCmd.Start()
}

func init() {
	rootCmd.AddCommand(openCmd)
}
