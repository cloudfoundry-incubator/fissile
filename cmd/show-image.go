package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagShowImageDockerOnly bool
	flagShowImageWithSizes  bool
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

		flagShowImageDockerOnly = viper.GetBool("docker-only")
		flagShowImageWithSizes = viper.GetBool("with-sizes")

		err := fissile.LoadReleases(
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
			flagRoleManifest,
			flagLightOpinions,
			flagDarkOpinions,
			flagShowImageDockerOnly,
			flagShowImageWithSizes,
			flagVerbose,
		)
	},
}

func init() {
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

	viper.BindPFlags(showImageCmd.PersistentFlags())
}
