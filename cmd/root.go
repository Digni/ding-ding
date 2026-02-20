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
	Long: `ding-ding sends notifications when AI agents (Claude, opencode, etc.) finish tasks.

It uses attention-aware 3-tier notifications:
- focused and active: quiet
- active but unfocused: system notification
- idle: system notification + push via ntfy, Discord, or webhooks.

Usage:
  ding-ding notify -m "Task completed"    Send a notification via CLI
  ding-ding serve                         Start HTTP server for agent POSTs
  ding-ding config init                   Create default config file`,
}

func Execute() {
	rootCmd.Version = Version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
