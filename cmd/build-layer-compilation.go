package cmd

import (
	"github.com/spf13/cobra"
)

var flagBuildLayerCompilationDebug bool

// buildLayerCompilationCmd represents the compilation command
var buildLayerCompilationCmd = &cobra.Command{
	Use:   "compilation",
	Short: "Builds a Docker image layer to be used when compiling packages.",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {

		err := fissile.CreateBaseCompilationImage(
			flagBuildLayerFrom,
			flagRepository,
			flagBuildLayerCompilationDebug,
		)

		return err
	},
}

func init() {
	buildLayerCmd.AddCommand(buildLayerCompilationCmd)

	buildLayerCompilationCmd.PersistentFlags().BoolVarP(
		&flagBuildLayerCompilationDebug,
		"debug",
		"D",
		false,
		"If specified, the docker container used to build the layer won't be destroyed on failure.",
	)
}
