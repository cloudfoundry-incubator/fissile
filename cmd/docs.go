package cmd

import (
	"github.com/spf13/cobra"
)

// docsCmd represents the docs command
var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Has subcommands to create documentation for fissile.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// We're simply overriding the root pre-run, since the docs commands
		// don't need it.
		return nil
	},
}

func init() {
	RootCmd.AddCommand(docsCmd)
}
