package cmd

import (
	"github.com/spf13/cobra"
)

// validateCmd represents the release command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validates all the configuration going into fissile.",
	Long: `
Displays a report of all validation checks.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := fissile.LoadManifest(
			flagRoleManifest,
			flagRelease,
			flagReleaseName,
			flagReleaseVersion,
			flagCacheDir,
		)
		if err != nil {
			return err
		}

		return fissile.Validate(flagLightOpinions, flagDarkOpinions)
	},
}

func init() {
	RootCmd.AddCommand(validateCmd)
}
