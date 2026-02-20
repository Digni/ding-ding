package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/spf13/cobra"
)

var Version = "dev"

var (
	configPathOverride string
	verboseMode        bool
)

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
		if isBestEffortNotifyError(err) {
			fmt.Fprintf(os.Stderr, "notification delivery failed: %v\n", err)
			return
		}

		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func isBestEffortNotifyError(err error) bool {
	var deliveryErr *notifyDeliveryError
	return errors.As(err, &deliveryErr)
}

func loadConfigForCommand() (config.LoadResult, error) {
	return config.LoadWithOptions(config.LoadOptions{
		ExplicitPath: configPathOverride,
		Warn: func(message string) {
			fmt.Fprintln(os.Stderr, message)
		},
	})
}

func printConfigSourceDetails(cmd *cobra.Command, source config.SourceSelection) {
	if !verboseMode {
		return
	}

	if source.Path != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "config source: %s (%s)\n", source.Path, source.Type)
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "config source: %s\n", source.Type)
	}
	if source.Reason != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "config reason: %s\n", source.Reason)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPathOverride, "config", "", "Path to config file")
	rootCmd.PersistentFlags().BoolVarP(&verboseMode, "verbose", "v", false, "Enable verbose output")
}
