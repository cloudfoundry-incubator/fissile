package cmd

import (
	"github.com/spf13/cobra"
)

// buildPackagesCmd represents the packages command
var buildPackagesCmd = &cobra.Command{
	Use:   "packages",
	Short: "Builds BOSH packages in a Docker container.",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {

		if err := fissile.LoadReleases(
			flagRelease,
			flagReleaseName,
			flagReleaseVersion,
			flagCacheDir,
		); err != nil {
			return err
		}

		return fissile.Compile(
			flagRepository,
			workPathCompilationDir,
			flagRoleManifest,
			flagWorkers,
		)
	},
}

func init() {
	buildCmd.AddCommand(buildPackagesCmd)
}
