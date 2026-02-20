package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "ding-ding",
	Short: "Agent completion notifications",
	Version: Version,
	Long: `ding-ding sends notifications when AI agents (Claude, opencode, etc.) finish tasks.

It shows a system notification immediately, and if you're away from your
computer (idle), it also pushes via ntfy, Discord, or custom webhooks.

Usage:
  ding-ding notify -m "Task completed"    Send a notification via CLI
  ding-ding serve                         Start HTTP server for agent POSTs
  ding-ding config init                   Create default config file`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
