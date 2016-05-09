package cmd

import (
	"github.com/spf13/cobra"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Prints a report with differences between two versions of a BOSH release.",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fissile.DiffConfigurationBases(
			flagRelease,
			flagCacheDir,
		)
	},
}

func init() {
	RootCmd.AddCommand(diffCmd)
}
