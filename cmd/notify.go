package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/notifier"
	"github.com/spf13/cobra"
)

var (
	notifyTitle   string
	notifyMessage string
	notifyAgent   string
	forcePush     bool
)

var notifyCmd = &cobra.Command{
	Use:   "notify [message]",
	Short: "Send a completion notification",
	Long: `Send a notification that an agent task has completed.

The message can be passed as a flag or as positional arguments:
  ding-ding notify -m "Build succeeded"
  ding-ding notify Build succeeded
  echo "done" | ding-ding notify

By default, a system notification is shown. If the user is idle beyond
the configured threshold, push notifications (ntfy, Discord, webhook)
are also sent. Use --push to always send push notifications.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		msg := notifier.Message{
			Title: notifyTitle,
			Agent: notifyAgent,
		}

		// Message priority: -m flag > positional args > stdin
		switch {
		case notifyMessage != "":
			msg.Body = notifyMessage
		case len(args) > 0:
			msg.Body = strings.Join(args, " ")
		default:
			// Check if stdin has data (piped input)
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				data, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<16))
				if err == nil && len(data) > 0 {
					msg.Body = strings.TrimSpace(string(data))
				}
			}
		}

		if msg.Body == "" {
			msg.Body = "Agent task completed"
		}

		if forcePush {
			// System notify + force push regardless of idle
			if err := notifier.Notify(cfg, msg); err != nil {
				fmt.Fprintf(os.Stderr, "notification error: %v\n", err)
			}
			return notifier.Push(cfg, msg)
		}

		return notifier.Notify(cfg, msg)
	},
}

func init() {
	notifyCmd.Flags().StringVarP(&notifyTitle, "title", "t", "ding ding!", "Notification title")
	notifyCmd.Flags().StringVarP(&notifyMessage, "message", "m", "", "Notification message")
	notifyCmd.Flags().StringVarP(&notifyAgent, "agent", "a", "", "Agent name (e.g. claude, opencode)")
	notifyCmd.Flags().BoolVarP(&forcePush, "push", "p", false, "Always send push notifications (ignore idle check)")

	rootCmd.AddCommand(notifyCmd)
}
