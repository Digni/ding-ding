package cmd

import (
	"fmt"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.Init()
		if err != nil {
			return err
		}
		fmt.Printf("Config created at %s\n", path)
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print config file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		source, err := config.ResolveConfigSource(config.ResolveOptions{})
		if err != nil {
			return err
		}

		path := source.Path
		if path == "" {
			path, err = config.ConfigPath()
			if err != nil {
				return err
			}
		}

		fmt.Println(path)
		printConfigSourceDetails(cmd, source)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
	rootCmd.AddCommand(configCmd)
}
