package cmd

import (
	"fmt"

	"github.com/Digni/ding-ding/internal/agentsetup"
	"github.com/spf13/cobra"
)

var (
	agentMode   string
	agentUpsert = agentsetup.Upsert
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agent integrations",
}

var agentInitCmd = &cobra.Command{
	Use:   "init <agent> <scope>",
	Short: "Initialize agent hooks/plugins",
	Args:  cobra.ExactArgs(2),
	RunE:  runAgentConfigure,
}

var agentUpdateCmd = &cobra.Command{
	Use:   "update <agent> <scope>",
	Short: "Update agent hooks/plugins",
	Args:  cobra.ExactArgs(2),
	RunE:  runAgentConfigure,
}

func runAgentConfigure(cmd *cobra.Command, args []string) error {
	agent, err := agentsetup.ParseAgent(args[0])
	if err != nil {
		return err
	}
	scope, err := agentsetup.ParseScope(args[1])
	if err != nil {
		return err
	}
	mode, err := agentsetup.ParseMode(agentMode)
	if err != nil {
		return err
	}

	result, err := agentUpsert(agentsetup.Options{
		Agent: agent,
		Scope: scope,
		Mode:  mode,
	})
	if err != nil {
		return fmt.Errorf("configure %s (%s): %w", agent, scope, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "configured %s (%s, mode=%s): %s [%s]\n", agent, scope, mode, result.Path, result.Status)
	return nil
}

func init() {
	agentCmd.PersistentFlags().StringVar(&agentMode, "mode", string(agentsetup.ModeCLI), "Integration mode: cli or server")
	agentCmd.AddCommand(agentInitCmd)
	agentCmd.AddCommand(agentUpdateCmd)
	rootCmd.AddCommand(agentCmd)
}
