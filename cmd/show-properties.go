package cmd

import (
	"github.com/SUSE/fissile/app"

	"github.com/spf13/cobra"
)

// showPropertiesCmd represents the properties command
var showPropertiesCmd = &cobra.Command{
	Use:   "properties",
	Short: "Displays information about BOSH properties, per jobs.",
	Long: `
Displays a report of all properties of all the jobs in the referenced releases.
The report lists the properties per job per release, with their default value.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Show property information

		err := fissile.LoadReleases(
			flagRelease,
			flagReleaseName,
			flagReleaseVersion,
			flagCacheDir,
		)
		if err != nil {
			return err
		}

		return fissile.ListProperties(app.OutputFormat(flagOutputFormat))
	},
}

func init() {
	showCmd.AddCommand(showPropertiesCmd)
}
