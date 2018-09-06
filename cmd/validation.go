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
		flagBuildHelmDefaultEnvFiles = splitNonEmpty(buildHelmViper.GetString("defaults-file"), ",")

		err := fissile.LoadReleases(
			flagRelease,
			flagReleaseName,
			flagReleaseVersion,
			flagCacheDir,
		)
		if err != nil {
			return err
		}

		return fissile.Validate(flagRoleManifest, flagLightOpinions, flagDarkOpinions, flagBuildHelmDefaultEnvFiles)
	},
}

func init() {

	validateCmd.PersistentFlags().StringP(
		"defaults-file",
		"D",
		"",
		"Env files that contain defaults for the configuration variables",
	)

	RootCmd.AddCommand(validateCmd)
}
