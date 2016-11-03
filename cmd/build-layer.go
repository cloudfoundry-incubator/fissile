package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagBuildLayerFrom    string
	flagBuildLayerNoBuild bool
)

// buildLayerCmd represents the layer command
var buildLayerCmd = &cobra.Command{
	Use:   "layer",
	Short: "Has subcommands for building Docker layers used during the creation of your images.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Inline the parts of the RootCmd.PersistentPreRunE we need.
		// Exclude the validateReleaseArgs(), this part we don't want.

		if err := validateBasicFlags(); err != nil {
			return err
		}

		flagBuildLayerFrom = viper.GetString("from")
		flagBuildLayerNoBuild = viper.GetBool("no-build")

		return nil
	},
}

func init() {
	buildCmd.AddCommand(buildLayerCmd)

	buildLayerCmd.PersistentFlags().StringP(
		"from",
		"F",
		"ubuntu:14.04",
		"Docker image used as a base for the layers",
	)

	buildLayerCmd.PersistentFlags().BoolP(
		"no-build",
		"N",
		false,
		"If specified, the Dockerfile and assets will be created, but the image won't be built.",
	)

	viper.BindPFlags(buildLayerCmd.PersistentFlags())
}
