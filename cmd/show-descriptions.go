package cmd

import (
	"github.com/spf13/cobra"
)

// showDescriptionsCmd represents the descriptions command
var showDescriptionsCmd = &cobra.Command{
	Use:   "descriptions",
	Short: "Displays descriptions of BOSH properties, per jobs.",
	Long: `
Displays a report of all properties of all the jobs in the referenced releases.
The report lists the properties per job per release, with their descriptions.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Show property information

		if err := fissile.LoadReleases(
			flagRelease,
			flagReleaseName,
			flagReleaseVersion,
			flagCacheDir,
		); err != nil {
			return err
		}

		return fissile.ListPropertyDescriptions(flagOutputFormat)
	},
}

func init() {
	showCmd.AddCommand(showDescriptionsCmd)
}
