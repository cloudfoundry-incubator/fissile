package cmd

import (
	"github.com/spf13/cobra"
)

// buildLayerStemcellCmd represents the runtime command
var buildLayerRuntimeCmd = &cobra.Command{
	Use:   "stemcell",
	Short: "Builds a Docker layer that is the base for all images",
	Long: `
This command creates a docker image to be used as a base layer for all role images,
similar to BOSH 'stemcells'.

Fissile will create a Dockerfile and a directory structure with all dependencies in 
` + "`<work-dir>/base_dockerfile`" + `. After that, it will build an image named 
` + "`<repository>-role-base:<FISSILE_VERSION>`" + `.
`,
	RunE: func(cmd *cobra.Command, args []string) error {

		return fissile.GenerateBaseDockerImage(
			workPathBaseDockerfile,
			flagBuildLayerFrom,
			flagMetrics,
			flagBuildLayerNoBuild,
			flagRepository,
		)
	},
}

func init() {
	buildLayerCmd.AddCommand(buildLayerRuntimeCmd)
}
