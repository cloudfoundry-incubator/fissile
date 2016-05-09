package cmd

import (
	"github.com/spf13/cobra"
)

var flagBuildImagesNoBuild bool
var flagBuildImagesForce bool

// buildImagesCmd represents the images command
var buildImagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Builds Docker images from your BOSH releases.",
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

		return fissile.GenerateRoleImages(
			workPathDockerDir,
			flagRepository,
			flagBuildImagesNoBuild,
			flagBuildImagesForce,
			flagRoleManifest,
			workPathCompilationDir,
			flagLightOpinions,
			flagDarkOpinions,
		)
	},
}

func init() {
	buildCmd.AddCommand(buildImagesCmd)

	buildImagesCmd.PersistentFlags().BoolVarP(
		&flagBuildImagesNoBuild,
		"no-build",
		"N",
		false,
		"If specified, the Dockerfile and assets will be created, but the image won't be built.",
	)

	buildImagesCmd.PersistentFlags().BoolVarP(
		&flagBuildImagesForce,
		"force",
		"F",
		false,
		"If specified, image creation will proceed even when images already exist.",
	)

}
