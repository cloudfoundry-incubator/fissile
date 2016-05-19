package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagBuildLayerCompilationDebug bool
)

// buildLayerCompilationCmd represents the compilation command
var buildLayerCompilationCmd = &cobra.Command{
	Use:   "compilation",
	Short: "Builds a docker image layer to be used when compiling packages.",
	Long: `
This command creates a container with the name ` + "`<repository>-cbase-<FISSILE_VERSION>`" + ` 
and runs a compilation prerequisites script within. 

Once the prerequisites script completes successfully, an image named 
` + "`<repository>-cbase:<FISSILE_VERSION>`" + ` is created and the created container is 
removed.

If the prerequisites script fails, the container is not removed. 
If the compilation base image already exists, this command does not do anything.
	`,
	RunE: func(cmd *cobra.Command, args []string) error {

		flagBuildLayerCompilationDebug = viper.GetBool("debug")

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

	buildLayerCompilationCmd.PersistentFlags().BoolP(
		"debug",
		"D",
		false,
		"If specified, the docker container used to build the layer won't be destroyed on failure.",
	)

	viper.BindPFlags(buildLayerCompilationCmd.PersistentFlags())
}
