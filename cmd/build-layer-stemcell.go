package cmd

import (
	"github.com/spf13/cobra"
)

// buildLayerStemcellCmd represents the runtime command
var buildLayerRuntimeCmd = &cobra.Command{
	Use:   "stemcell",
	Short: "Builds a Docker layer that is the base for all images",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {

		return fissile.GenerateBaseDockerImage(
			workPathBaseDockerfile,
			flagConfiggin,
			flagBuildLayerFrom,
			flagBuildLayerNoBuild,
			flagRepository,
		)
	},
}

func init() {
	buildLayerCmd.AddCommand(buildLayerRuntimeCmd)
}
