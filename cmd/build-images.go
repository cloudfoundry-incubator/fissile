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
	Long: `
This command goes through all the role definitions in the role manifest creating a
Dockerfile for each of them and building it.

Each role gets a directory ` + "`<work-dir>/dockerfiles`" + `. In each directory one can find 
a Dockerfile and a directory structure that gets ADDed to the docker image. The
directory structure contains jobs, packages and all other necessary scripts and 
templates.

The images will have a 'role' label useful for filtering.
The entrypoint for each image is ` + "`/opt/hcf/run.sh`" + `.

Before running this command, you should run ` + "`fissile build layer stemcell`" + `.

The images will be tagged: ` + "`<repository>-<role_name>:<SIGNATURE>`" + `.
The SIGNATURE is based on the hashes of all jobs and packages that are included in
the image.
	`,
	RunE: func(cmd *cobra.Command, args []string) error {

		flagBuildImagesNoBuild = viper.GetBool("no-build")
		flagBuildImagesForce = viper.GetBool("force")

		err := fissile.LoadReleases(
			flagRelease,
			flagReleaseName,
			flagReleaseVersion,
			flagCacheDir,
		)
		if err != nil {
			return err
		}

		return fissile.GenerateRoleImages(
			workPathDockerDir,
			flagRepository,
			flagMetrics,
			flagBuildImagesNoBuild,
			flagBuildImagesForce,
			flagWorkers,
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
