package cmd

import (
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays fissile's version.",
	Run: func(cmd *cobra.Command, args []string) {
		fissile.UI.Printf("%s\n", version)
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// We're simply overriding the root pre-run, since the docs commands
		// don't need it.
		return nil
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
