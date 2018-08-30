package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagShowImageDockerOnly bool
	flagShowImageWithSizes  bool
	flagShowImageTagExtra   string
)

// showImageCmd represents the image command
var showImageCmd = &cobra.Command{
	Use:   "image",
	Short: "Displays information about role images.",
	Long: `
This command lists all the final docker image names for all the roles defined in 
your role manifest.

This command is useful in conjunction with docker (e.g. ` + "`docker rmi $(fissile show image)`" + `).
`,
	RunE: func(cmd *cobra.Command, args []string) error {

		flagShowImageDockerOnly = showImagesViper.GetBool("docker-only")
		flagShowImageWithSizes = showImagesViper.GetBool("with-sizes")
		flagShowImageTagExtra = showImagesViper.GetString("tag-extra")

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

		return fissile.ListRoleImages(
			flagDockerRegistry,
			flagDockerOrganization,
			flagRepository,
			flagLightOpinions,
			flagDarkOpinions,
			flagShowImageDockerOnly,
			flagShowImageWithSizes,
			flagShowImageTagExtra,
		)
	},
}

var showImagesViper = viper.New()

func init() {
	initViper(showImagesViper)

	showCmd.AddCommand(showImageCmd)

	showImageCmd.PersistentFlags().BoolP(
		"docker-only",
		"D",
		false,
		"If the flag is set, only show images that are available on docker",
	)

	showImageCmd.PersistentFlags().BoolP(
		"with-sizes",
		"S",
		false,
		"If the flag is set, also show image virtual sizes; only works if the --docker-only flag is set",
	)

	showImageCmd.PersistentFlags().StringP(
		"tag-extra",
		"",
		"",
		"Additional information to use in computing the image tags",
	)

	showImagesViper.BindPFlags(showImageCmd.PersistentFlags())
}
