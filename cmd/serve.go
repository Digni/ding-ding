package cmd

import (
	"fmt"

	"github.com/Digni/ding-ding/internal/server"
	"github.com/spf13/cobra"
)

var serveAddress string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server for agent notifications",
	Long: `Start an HTTP server that agents can POST to when tasks complete.

Endpoints:
  POST /notify    Send notification (JSON body: {"title":"...", "body":"...", "agent":"..."})
  GET  /notify    Quick notify (?title=...&message=...&agent=...)
  GET  /health    Health check

Example:
  curl -X POST localhost:8228/notify -d '{"body":"Build done","agent":"claude"}'
  curl "localhost:8228/notify?message=done&agent=claude"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		loadResult, err := loadConfigForCommand()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		printConfigSourceDetails(cmd, loadResult.Source)
		cfg := loadResult.Config

		if serveAddress != "" {
			cfg.Server.Address = serveAddress
		}

		return server.Start(cfg)
	},
}

func init() {
	serveCmd.Flags().StringVarP(&serveAddress, "address", "l", "", "Listen address (default from config, e.g. :8228)")

	rootCmd.AddCommand(serveCmd)
}
