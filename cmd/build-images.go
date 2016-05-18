package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagBuildImagesNoBuild bool
	flagBuildImagesForce   bool
)

// buildImagesCmd represents the images command
var buildImagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Builds Docker images from your BOSH releases.",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {

		flagBuildImagesNoBuild = viper.GetBool("no-build")
		flagBuildImagesForce = viper.GetBool("force")

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

	buildImagesCmd.PersistentFlags().BoolP(
		"no-build",
		"N",
		false,
		"If specified, the Dockerfile and assets will be created, but the image won't be built.",
	)

	buildImagesCmd.PersistentFlags().BoolP(
		"force",
		"F",
		false,
		"If specified, image creation will proceed even when images already exist.",
	)

	viper.BindPFlags(buildImagesCmd.PersistentFlags())
}
