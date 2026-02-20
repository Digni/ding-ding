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
	testLocal     bool
)

var notifyCmd = &cobra.Command{
	Use:   "notify [message]",
	Short: "Send a completion notification",
	Long: `Send a notification that an agent task has completed.

The message can be passed as a flag or as positional arguments:
  ding-ding notify -m "Build succeeded"
  ding-ding notify Build succeeded
  echo "done" | ding-ding notify

By default, focused terminals are quiet, active unfocused sends a system
notification, and idle sends system + push notifications.

Use --push to force remote push (ntfy/Discord/webhook) regardless of
focus/idle, and --test-local to force a local/system notification even
when focus suppression would normally silence it.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if hasMistypedTestLocalArg(os.Args[1:]) {
			return fmt.Errorf("invalid flag -test-local; use --test-local")
		}

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

		return notifier.NotifyWithOptions(cfg, msg, notifier.NotifyOptions{
			ForcePush:  forcePush,
			ForceLocal: testLocal,
		})
	},
}

func init() {
	notifyCmd.Flags().StringVarP(&notifyTitle, "title", "t", "ding ding!", "Notification title")
	notifyCmd.Flags().StringVarP(&notifyMessage, "message", "m", "", "Notification message")
	notifyCmd.Flags().StringVarP(&notifyAgent, "agent", "a", "", "Agent name (e.g. claude, opencode)")
	notifyCmd.Flags().BoolVarP(&forcePush, "push", "p", false, "Always send push notifications (ignore idle/focus for remote backends)")
	notifyCmd.Flags().BoolVar(&testLocal, "test-local", false, "Always send a local/system notification (ignore focused suppression)")

	rootCmd.AddCommand(notifyCmd)
}

func hasMistypedTestLocalArg(argv []string) bool {
	expectsValue := false

	for _, arg := range argv {
		if expectsValue {
			expectsValue = false
			continue
		}

		if arg == "--" {
			return false
		}

		switch arg {
		case "-m", "--message", "-t", "--title", "-a", "--agent":
			expectsValue = true
			continue
		}

		if arg == "-test-local" || strings.HasPrefix(arg, "-test-local=") {
			return true
		}
	}

	return false
}
