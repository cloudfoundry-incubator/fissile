package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// showImageCmd represents the image command
var showImageCmd = &cobra.Command{
	Use:   "image",
	Short: "Displays information about instance group images.",
	Long: `
This command lists all the final docker image names for all the instance groups defined in
your role manifest.

This command is useful in conjunction with docker (e.g. ` + "`docker rmi $(fissile show image)`" + `).
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := fissile.LoadManifest()
		if err != nil {
			return err
		}

		return fissile.ListRoleImages(
			showImagesViper.GetBool("docker-only"),
			showImagesViper.GetBool("with-sizes"),
			showImagesViper.GetString("tag-extra"),
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
