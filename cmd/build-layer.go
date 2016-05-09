package cmd

import (
	"github.com/spf13/cobra"
)

var flagBuildLayerFrom string
var flagBuildLayerNoBuild bool

// buildLayerCmd represents the layer command
var buildLayerCmd = &cobra.Command{
	Use:   "layer",
	Short: "Has subcommands for building Docker layers used during the creation of your images.",
	Long:  ``,
}

func init() {
	buildCmd.AddCommand(buildLayerCmd)

	buildLayerCmd.PersistentFlags().StringVarP(
		&flagBuildLayerFrom,
		"from",
		"F",
		"ubuntu:14.04",
		"Docker image used as a base for the layers",
	)

	buildLayerCmd.PersistentFlags().BoolVarP(
		&flagBuildLayerNoBuild,
		"no-build",
		"N",
		false,
		"If specified, the Dockerfile and assets will be created, but the image won't be built.",
	)
}
