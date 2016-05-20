package cmd

import (
	"github.com/spf13/cobra"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Prints a report with differences between two versions of a BOSH release.",
	Long: `
This command goes through all BOSH job configuration parameters for two versions of
the same release and displays all the changes it can find (which keys were dropped, 
which added, and which had their default values changed).
`,
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
